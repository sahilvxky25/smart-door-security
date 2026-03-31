from flask import Flask, request, jsonify
import cv2
import numpy as np
import os
import re
import time
import threading
import traceback
import requests as http_requests
import base64
import pickle
import torch
from facenet_pytorch import MTCNN, InceptionResnetV1
from PIL import Image
from anti_spoof import AntiSpoof

app = Flask(__name__)

# Prevents concurrent camera access from overlapping PIR triggers
camera_lock = threading.Lock()
# Protects known_faces dict from concurrent enrollment/recognition
faces_lock = threading.RLock()

KNOWN_FACES_DIR = "face_service/known_faces"
EMBEDDINGS_FILE = "face_service/embeddings.pkl"
MATCH_THRESHOLD = 0.65  # cosine similarity for FaceNet embeddings
CAPTURE_DURATION = 2.5  # seconds of video to capture for liveness
CAPTURE_FPS = 12  # target frame rate during capture

# Allowed name characters: letters, digits, spaces, hyphens, underscores
_NAME_RE = re.compile(r'^[\w\s\-]{1,64}$')

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

# MTCNN with keep_all=True to detect multiple faces (used in enrollment validation)
mtcnn_all = MTCNN(
    image_size=160,
    margin=20,
    keep_all=True,
    device=device,
)

facenet = InceptionResnetV1(pretrained="vggface2").eval().to(device)

anti_spoof = AntiSpoof()

# Store {name: 512-d embedding tensor}
known_faces = {}


# ----------------------------
# PERSISTENCE
# ----------------------------
def save_embeddings():
    """Persist embeddings dict to disk."""
    try:
        with faces_lock:
            data = {name: emb.cpu() for name, emb in known_faces.items()}
        with open(EMBEDDINGS_FILE, "wb") as f:
            pickle.dump(data, f)
        print(f"[FaceService] Saved {len(data)} embeddings to {EMBEDDINGS_FILE}")
    except Exception as e:
        print(f"[FaceService] WARNING: failed to save embeddings: {e}")


def load_known_faces():
    """Load embeddings from pickle (fast) or fall back to directory scan."""
    os.makedirs(KNOWN_FACES_DIR, exist_ok=True)

    # Try loading from pickle first
    if os.path.exists(EMBEDDINGS_FILE):
        try:
            with open(EMBEDDINGS_FILE, "rb") as f:
                data = pickle.load(f)
            with faces_lock:
                for name, emb in data.items():
                    known_faces[name] = emb.to(device)
            print(f"[FaceService] Loaded {len(known_faces)} embeddings from pickle")
            return
        except Exception as e:
            print(f"[FaceService] Pickle load failed ({e}), falling back to directory scan")

    # Fall back to scanning known_faces directory
    loaded = 0
    for filename in os.listdir(KNOWN_FACES_DIR):
        if not filename.lower().endswith((".jpg", ".jpeg", ".png")):
            continue
        path = os.path.join(KNOWN_FACES_DIR, filename)
        try:
            image = Image.open(path).convert("RGB")
            face_tensor = mtcnn(image)
            if face_tensor is None:
                print(f"[FaceService] WARNING: no face detected in {filename}, skipping")
                continue
            with torch.no_grad():
                embedding = facenet(face_tensor.unsqueeze(0).to(device))
            name = os.path.splitext(filename)[0]
            with faces_lock:
                known_faces[name] = embedding[0]
            loaded += 1
            print(f"[FaceService] Loaded: {name} (from {filename})")
        except Exception as e:
            print(f"[FaceService] WARNING: failed to load {filename}: {e}")

    print(f"[FaceService] {len(known_faces)} known faces ready: {list(known_faces.keys())}")
    if loaded > 0:
        save_embeddings()


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

    with faces_lock:
        faces_snapshot = dict(known_faces)

    for name, known_emb in faces_snapshot.items():
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


@app.route("/recognize-url", methods=["POST"])
def recognize_url():
    """Recognize a face from an image URL (e.g. Cloudinary).

    Accepts JSON: {"url": "https://res.cloudinary.com/..."}
    Downloads the image and runs face recognition (no anti-spoof).
    """
    try:
        data = request.get_json(silent=True)
        if not data:
            return jsonify({"error": "request body must be JSON"}), 400

        url = (data.get("url") or "").strip()
        if not url:
            return jsonify({"error": "url is required"}), 400

        # Download image from URL
        try:
            resp = http_requests.get(url, timeout=10)
            resp.raise_for_status()
        except Exception as e:
            return jsonify({"error": f"failed to download image from URL: {str(e)}"}), 400

        image_bytes = resp.content
        if not image_bytes:
            return jsonify({"error": "downloaded image is empty"}), 400

        npimg = np.frombuffer(image_bytes, np.uint8)
        frame = cv2.imdecode(npimg, cv2.IMREAD_COLOR)
        if frame is None:
            return jsonify({"error": "could not decode downloaded image"}), 400

        match, user = recognize_face(frame)
        return jsonify({"match": match, "user": user})

    except Exception as e:
        traceback.print_exc()
        return jsonify({"error": f"internal error: {str(e)}"}), 500


@app.route("/capture-and-recognize", methods=["POST"])
def capture_and_recognize():
    """Full pipeline: capture video -> anti-spoof -> face recognition.
    This is the primary endpoint called by the Go backend on PIR trigger."""
    try:
        with faces_lock:
            n_faces = len(known_faces)
        if n_faces == 0:
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


@app.route("/enroll", methods=["POST"])
def enroll():
    """Enroll a new face.

    Accepts JSON: {"name": "Alice", "image": "<base64-encoded JPEG/PNG>"}

    Edge cases handled:
    - Empty / invalid name
    - Corrupt or non-image data
    - No face detected in image
    - Multiple faces detected (ambiguous)
    - Name already enrolled (overwrite)
    - Concurrent enrollment (serialized via faces_lock)
    """
    try:
        data = request.get_json(silent=True)
        if not data:
            return jsonify({"error": "request body must be JSON"}), 400

        name = (data.get("name") or "").strip()
        image_b64 = data.get("image") or ""

        # Validate name
        if not name:
            return jsonify({"error": "name is required"}), 400
        if not _NAME_RE.match(name):
            return jsonify({"error": "name may only contain letters, digits, spaces, hyphens and underscores (max 64 chars)"}), 400

        # Decode image
        if not image_b64:
            return jsonify({"error": "image is required (base64-encoded)"}), 400
        try:
            image_bytes = base64.b64decode(image_b64)
        except Exception:
            return jsonify({"error": "image is not valid base64"}), 400

        npimg = np.frombuffer(image_bytes, np.uint8)
        frame = cv2.imdecode(npimg, cv2.IMREAD_COLOR)
        if frame is None:
            return jsonify({"error": "could not decode image — unsupported format or corrupt data"}), 400

        rgb = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
        pil_image = Image.fromarray(rgb)

        # Check for multiple faces (reject ambiguous enrollment)
        all_faces = mtcnn_all(pil_image)
        if all_faces is None:
            return jsonify({"error": "no face detected in the provided image"}), 422
        if isinstance(all_faces, list) or (hasattr(all_faces, 'shape') and all_faces.ndim == 4 and all_faces.shape[0] > 1):
            n = all_faces.shape[0] if hasattr(all_faces, 'shape') else len(all_faces)
            if n > 1:
                return jsonify({"error": f"multiple faces detected ({n}) — please provide an image with exactly one face"}), 422

        # Get single face tensor
        face_tensor = mtcnn(pil_image)
        if face_tensor is None:
            return jsonify({"error": "face detection failed — image may be too blurry or face too small"}), 422

        # Compute embedding
        with torch.no_grad():
            embedding = facenet(face_tensor.unsqueeze(0).to(device))[0]

        overwrite = name in known_faces

        # Save image to disk
        safe_filename = name.replace(" ", "_") + ".jpg"
        image_path = os.path.join(KNOWN_FACES_DIR, safe_filename)
        cv2.imwrite(image_path, frame)

        # Update in-memory dict and persist
        with faces_lock:
            known_faces[name] = embedding
        save_embeddings()

        print(f"[FaceService] Enrolled: {name!r} (overwrite={overwrite})")
        return jsonify({
            "enrolled": name,
            "overwrite": overwrite,
            "total_known": len(known_faces),
        }), 200

    except Exception as e:
        traceback.print_exc()
        return jsonify({"error": f"internal error: {str(e)}"}), 500


@app.route("/faces", methods=["GET"])
def list_faces():
    """List all enrolled face names."""
    with faces_lock:
        names = list(known_faces.keys())
    return jsonify({"faces": names, "count": len(names)})


@app.route("/faces/<name>", methods=["DELETE"])
def delete_face(name):
    """Remove an enrolled face by name."""
    name = name.strip()
    with faces_lock:
        if name not in known_faces:
            return jsonify({"error": f"face {name!r} not found"}), 404
        del known_faces[name]

    # Remove image from disk (best-effort)
    for ext in (".jpg", ".jpeg", ".png"):
        path = os.path.join(KNOWN_FACES_DIR, name.replace(" ", "_") + ext)
        if os.path.exists(path):
            try:
                os.remove(path)
            except Exception as e:
                print(f"[FaceService] WARNING: could not delete image {path}: {e}")

    save_embeddings()
    print(f"[FaceService] Deleted face: {name!r}")
    return jsonify({"deleted": name, "total_known": len(known_faces)})


@app.route("/health", methods=["GET"])
def health():
    with faces_lock:
        n = len(known_faces)
        names = list(known_faces.keys())
    return jsonify({
        "status": "ok",
        "model": "FaceNet (InceptionResnetV1 + VGGFace2)",
        "anti_spoof": "blink + head_movement + texture (2/3 required)",
        "device": str(device),
        "known_faces": n,
        "known_users": names,
    })


# ----------------------------
# START SERVER
# ----------------------------
if __name__ == "__main__":
    load_known_faces()
    app.run(host="0.0.0.0", port=5000)
