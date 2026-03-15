from flask import Flask, request, jsonify
import cv2
import numpy as np
import os
import time
import threading
import traceback
import base64
import torch
from facenet_pytorch import MTCNN, InceptionResnetV1
from PIL import Image
from anti_spoof import AntiSpoof

app = Flask(__name__)

# Prevents concurrent camera access from overlapping PIR triggers
camera_lock = threading.Lock()

KNOWN_FACES_DIR = "face_service/known_faces"
MATCH_THRESHOLD = 0.65  # cosine similarity for FaceNet embeddings
CAPTURE_DURATION = 2.5  # seconds of video to capture for liveness
CAPTURE_FPS = 12  # target frame rate during capture

# ----------------------------
# INITIALIZE MODELS
# ----------------------------
device = torch.device("cuda" if torch.cuda.is_available() else "cpu")

mtcnn = MTCNN(
    image_size=160,
    margin=20,
    keep_all=False,
    device=device,
)

facenet = InceptionResnetV1(pretrained="vggface2").eval().to(device)

anti_spoof = AntiSpoof()

# Store {name: 512-d embedding tensor}
known_faces = {}


# ----------------------------
# LOAD KNOWN FACES
# ----------------------------
def load_known_faces():
    for filename in os.listdir(KNOWN_FACES_DIR):
        path = os.path.join(KNOWN_FACES_DIR, filename)

        image = Image.open(path).convert("RGB")
        face_tensor = mtcnn(image)

        if face_tensor is None:
            print(f"[FaceService] WARNING: no face detected in {filename}, skipping")
            continue

        with torch.no_grad():
            embedding = facenet(face_tensor.unsqueeze(0).to(device))

        name = os.path.splitext(filename)[0]
        known_faces[name] = embedding[0]
        print(f"[FaceService] Loaded: {name} (from {filename})")

    print(f"[FaceService] {len(known_faces)} known faces ready: {list(known_faces.keys())}")


# ----------------------------
# COSINE SIMILARITY
# ----------------------------
def cosine_similarity(a, b):
    return float(torch.nn.functional.cosine_similarity(a.unsqueeze(0), b.unsqueeze(0)))


# ----------------------------
# RECOGNIZE FACE
# ----------------------------
def recognize_face(frame):
    """Run MTCNN + FaceNet on a single frame.
    Returns (matched, name)."""
    rgb = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
    image = Image.fromarray(rgb)

    face_tensor = mtcnn(image)
    if face_tensor is None:
        print("[FaceService] No face detected in recognition frame")
        return False, ""

    with torch.no_grad():
        embedding = facenet(face_tensor.unsqueeze(0).to(device))[0]

    best_score = -1.0
    best_name = ""

    for name, known_emb in known_faces.items():
        score = cosine_similarity(embedding, known_emb)
        print(f"[FaceService]   vs {name}: similarity={score:.4f}")
        if score > best_score:
            best_score = score
            best_name = name

    print(
        f"[FaceService] Best match: {best_name} "
        f"(similarity={best_score:.4f}, threshold={MATCH_THRESHOLD})"
    )

    if best_score >= MATCH_THRESHOLD:
        return True, best_name
    return False, ""


# ----------------------------
# VIDEO SEQUENCE CAPTURE
# ----------------------------
def capture_video_sequence(duration=CAPTURE_DURATION, fps=CAPTURE_FPS):
    """Capture multiple frames over a time window for liveness detection.
    Uses a lock to prevent concurrent camera access from overlapping PIR triggers.
    Returns a list of BGR frames."""

    if not camera_lock.acquire(timeout=5):
        print("[FaceService] Camera is busy (another capture in progress)")
        return []

    try:
        cap = cv2.VideoCapture(0, cv2.CAP_DSHOW)
        if not cap.isOpened():
            print("[FaceService] Failed to open camera")
            return []

        # Warmup: discard first few frames (Windows cameras need this)
        for _ in range(3):
            cap.read()

        frames = []
        interval = 1.0 / fps
        start = time.time()
        last_capture = 0.0

        print(f"[FaceService] Capturing {duration}s of video at ~{fps} fps ...")

        while time.time() - start < duration:
            ret, frame = cap.read()
            if not ret:
                break

            now = time.time()
            if now - last_capture >= interval:
                frames.append(frame)
                last_capture = now

        cap.release()
        print(f"[FaceService] Captured {len(frames)} frames in {time.time() - start:.1f}s")
        return frames
    except Exception as e:
        print(f"[FaceService] Camera error: {e}")
        return []
    finally:
        camera_lock.release()


# ----------------------------
# API ENDPOINTS
# ----------------------------
@app.route("/recognize", methods=["POST"])
def recognize():
    """Recognize a face from raw image bytes (no anti-spoof)."""
    image_bytes = request.data
    if not image_bytes:
        return jsonify({"error": "no image data received"}), 400

    npimg = np.frombuffer(image_bytes, np.uint8)
    frame = cv2.imdecode(npimg, cv2.IMREAD_COLOR)
    if frame is None:
        return jsonify({"error": "could not decode image"}), 400

    match, user = recognize_face(frame)
    return jsonify({"match": match, "user": user})


@app.route("/capture-and-recognize", methods=["POST"])
def capture_and_recognize():
    """Full pipeline: capture video -> anti-spoof -> face recognition.
    This is the primary endpoint called by the Go backend on PIR trigger."""
    try:
        if len(known_faces) == 0:
            return jsonify({"error": "no known faces loaded"}), 503

        # Step 1: Capture video sequence
        frames = capture_video_sequence()
        if len(frames) < 5:
            return jsonify({"error": f"not enough frames captured ({len(frames)}), camera may be busy"}), 500

        # Step 2: Anti-spoof liveness checks
        alive, spoof_results = anti_spoof.run(frames)

        if not alive:
            print("[FaceService] SPOOF DETECTED - rejecting")
            mid_frame = frames[len(frames) // 2]
            _, buf = cv2.imencode(".jpg", mid_frame)
            frame_b64 = base64.b64encode(buf.tobytes()).decode("ascii")
            return jsonify({
                "match": False,
                "user": "",
                "spoof": True,
                "anti_spoof": spoof_results,
                "frame": frame_b64,
            })

        # Step 3: Face recognition on the middle frame (most stable)
        best_frame = frames[len(frames) // 2]
        match, user = recognize_face(best_frame)

        _, buf = cv2.imencode(".jpg", best_frame)
        frame_b64 = base64.b64encode(buf.tobytes()).decode("ascii")

        print(f"[FaceService] capture-and-recognize -> alive=True match={match} user={user!r}")
        return jsonify({
            "match": match,
            "user": user,
            "spoof": False,
            "anti_spoof": spoof_results,
            "frame": frame_b64,
        })

    except Exception as e:
        traceback.print_exc()
        return jsonify({"error": f"internal error: {str(e)}"}), 500


@app.route("/health", methods=["GET"])
def health():
    return jsonify({
        "status": "ok",
        "model": "FaceNet (InceptionResnetV1 + VGGFace2)",
        "anti_spoof": "blink + head_movement + texture (2/3 required)",
        "device": str(device),
        "known_faces": len(known_faces),
        "known_users": list(known_faces.keys()),
    })


# ----------------------------
# START SERVER
# ----------------------------
if __name__ == "__main__":
    load_known_faces()
    app.run(host="0.0.0.0", port=5000)
