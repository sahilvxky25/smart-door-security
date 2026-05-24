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

# Prevents concurrent camera access from overlapping PIR triggers.
camera_lock = threading.Lock()
# Protects known_faces from concurrent enrollment/recognition.
faces_lock = threading.RLock()

KNOWN_FACES_DIR = "face_service/known_faces"
EMBEDDINGS_FILE = "face_service/embeddings.pkl"
MATCH_THRESHOLD = 0.65
CAPTURE_DURATION = 2.5
CAPTURE_FPS = 12

_NAME_RE = re.compile(r"^[\w\s\-]{1,64}$")

device = torch.device("cuda" if torch.cuda.is_available() else "cpu")

mtcnn = MTCNN(
    image_size=160,
    margin=20,
    keep_all=False,
    device=device,
)

mtcnn_all = MTCNN(
    image_size=160,
    margin=20,
    keep_all=True,
    device=device,
)

facenet = InceptionResnetV1(pretrained="vggface2").eval().to(device)
anti_spoof = AntiSpoof()

# {user_id: {member_id: {"embedding": tensor, "name": str, "member_id": str}}}
known_faces = {}


def normalize_id(value):
    return str(value or "").strip()


def parse_legacy_face_name(name):
    parts = str(name).split("_", 1)
    if len(parts) == 2 and parts[0].isdigit():
        return parts[0], str(name), parts[1]
    return "legacy", str(name), str(name)


def save_embeddings():
    try:
        with faces_lock:
            data = {}
            for user_id, faces in known_faces.items():
                data[str(user_id)] = {}
                for member_id, record in faces.items():
                    data[str(user_id)][str(member_id)] = {
                        "embedding": record["embedding"].detach().cpu(),
                        "name": record.get("name", ""),
                        "member_id": str(record.get("member_id", member_id)),
                    }

        with open(EMBEDDINGS_FILE, "wb") as f:
            pickle.dump(data, f)

        total = sum(len(faces) for faces in data.values())
        print(f"[FaceService] Saved {total} scoped embeddings to {EMBEDDINGS_FILE}")
    except Exception as e:
        print(f"[FaceService] WARNING: failed to save embeddings: {e}")


def load_known_faces():
    os.makedirs(KNOWN_FACES_DIR, exist_ok=True)

    if os.path.exists(EMBEDDINGS_FILE):
        try:
            with open(EMBEDDINGS_FILE, "rb") as f:
                data = pickle.load(f)

            with faces_lock:
                for key, value in data.items():
                    if isinstance(value, dict):
                        for member_id, record in value.items():
                            if isinstance(record, dict) and "embedding" in record:
                                embedding = record["embedding"]
                                name = record.get("name", str(member_id))
                                stored_member_id = normalize_id(record.get("member_id")) or str(member_id)
                            else:
                                embedding = record
                                name = str(member_id)
                                stored_member_id = str(member_id)

                            known_faces.setdefault(str(key), {})[stored_member_id] = {
                                "embedding": embedding.to(device),
                                "name": name,
                                "member_id": stored_member_id,
                            }
                    else:
                        user_id, member_id, name = parse_legacy_face_name(key)
                        known_faces.setdefault(user_id, {})[member_id] = {
                            "embedding": value.to(device),
                            "name": name,
                            "member_id": member_id,
                        }

            total = sum(len(faces) for faces in known_faces.values())
            print(f"[FaceService] Loaded {total} scoped embeddings from pickle")
            return
        except Exception as e:
            print(f"[FaceService] Pickle load failed ({e}), falling back to directory scan")

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
                embedding = facenet(face_tensor.unsqueeze(0).to(device))[0]

            name = os.path.splitext(filename)[0]
            user_id, member_id, display_name = parse_legacy_face_name(name)
            with faces_lock:
                known_faces.setdefault(user_id, {})[member_id] = {
                    "embedding": embedding,
                    "name": display_name,
                    "member_id": member_id,
                }
            loaded += 1
            print(f"[FaceService] Loaded user={user_id} member={member_id} from {filename}")
        except Exception as e:
            print(f"[FaceService] WARNING: failed to load {filename}: {e}")

    total = sum(len(faces) for faces in known_faces.values())
    print(f"[FaceService] {total} known faces ready across {len(known_faces)} users")
    if loaded > 0:
        save_embeddings()


def cosine_similarity(a, b):
    return float(torch.nn.functional.cosine_similarity(a.unsqueeze(0), b.unsqueeze(0)))


def embedding_to_list(embedding):
    return [float(x) for x in embedding.detach().cpu().tolist()]


def embedding_from_list(values):
    return torch.tensor(values, dtype=torch.float32, device=device)


def load_image_from_url(url):
    resp = http_requests.get(url, timeout=10)
    resp.raise_for_status()
    if not resp.content:
        raise ValueError("downloaded image is empty")

    img_array = np.asarray(bytearray(resp.content), dtype=np.uint8)
    image = cv2.imdecode(img_array, cv2.IMREAD_COLOR)
    if image is None:
        raise ValueError("could not decode downloaded image")
    return image


def validate_single_face_and_embed(frame):
    rgb = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
    pil_image = Image.fromarray(rgb)

    all_faces = mtcnn_all(pil_image)
    if all_faces is None:
        return None, ("no face detected in the provided image", 422)

    if isinstance(all_faces, list) or (
        hasattr(all_faces, "shape") and all_faces.ndim == 4 and all_faces.shape[0] > 1
    ):
        n = all_faces.shape[0] if hasattr(all_faces, "shape") else len(all_faces)
        if n > 1:
            return None, (f"multiple faces detected ({n}); provide exactly one face", 422)

    face_tensor = mtcnn(pil_image)
    if face_tensor is None:
        return None, ("face detection failed; image may be blurry or face too small", 422)

    with torch.no_grad():
        embedding = facenet(face_tensor.unsqueeze(0).to(device))[0]
    return embedding, None


def enroll_frame(user_id, member_id, name, frame):
    user_id = normalize_id(user_id)
    member_id = normalize_id(member_id)
    name = (name or "").strip()

    if not user_id:
        return None, ("user_id is required", 400)
    if not member_id:
        return None, ("member_id is required", 400)
    if not name:
        return None, ("name is required", 400)
    if not _NAME_RE.match(name):
        return None, ("name may only contain letters, digits, spaces, hyphens and underscores (max 64 chars)", 400)

    embedding, error = validate_single_face_and_embed(frame)
    if error:
        return None, error

    with faces_lock:
        user_faces = known_faces.setdefault(user_id, {})
        overwrite = member_id in user_faces
        user_faces[member_id] = {
            "embedding": embedding,
            "name": name,
            "member_id": member_id,
        }
        total_known = len(user_faces)

    save_embeddings()
    return {"overwrite": overwrite, "total_known": total_known}, None


def recognize_face(frame, user_id):
    user_id = normalize_id(user_id)
    if not user_id:
        return False, "", "", -1.0

    rgb = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
    image = Image.fromarray(rgb)

    face_tensor = mtcnn(image)
    if face_tensor is None:
        print("[FaceService] No face detected in recognition frame")
        return False, "", "", -1.0

    with torch.no_grad():
        embedding = facenet(face_tensor.unsqueeze(0).to(device))[0]

    with faces_lock:
        faces_snapshot = dict(known_faces.get(user_id, {}))

    if not faces_snapshot:
        print(f"[FaceService] No enrolled faces for user_id={user_id}")
        return False, "", "", -1.0

    best_score = -1.0
    best_name = ""
    best_member_id = ""

    for member_id, record in faces_snapshot.items():
        score = cosine_similarity(embedding, record["embedding"])
        display_name = record.get("name", str(member_id))
        print(f"[FaceService] user={user_id} vs {display_name}: similarity={score:.4f}")
        if score > best_score:
            best_score = score
            best_name = display_name
            best_member_id = str(record.get("member_id", member_id))

    print(
        f"[FaceService] Best scoped match user={user_id}: {best_name} "
        f"(similarity={best_score:.4f}, threshold={MATCH_THRESHOLD})"
    )

    if best_score >= MATCH_THRESHOLD:
        return True, best_name, best_member_id, best_score
    return False, "", "", best_score


def recognize_face_candidates(frame, candidates):
    rgb = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
    image = Image.fromarray(rgb)

    face_tensor = mtcnn(image)
    if face_tensor is None:
        print("[FaceService] No face detected in recognition frame")
        return False, "", "", -1.0

    with torch.no_grad():
        embedding = facenet(face_tensor.unsqueeze(0).to(device))[0]

    best_score = -1.0
    best_name = ""
    best_member_id = ""

    for candidate in candidates or []:
        raw_embedding = candidate.get("embedding") or []
        if not raw_embedding:
            continue
        known_emb = embedding_from_list(raw_embedding)
        score = cosine_similarity(embedding, known_emb)
        display_name = str(candidate.get("name") or "")
        member_id = str(candidate.get("member_id") or "")
        print(f"[FaceService] candidate member={member_id} name={display_name}: similarity={score:.4f}")
        if score > best_score:
            best_score = score
            best_name = display_name
            best_member_id = member_id

    print(
        f"[FaceService] Best candidate match: {best_name} "
        f"(similarity={best_score:.4f}, threshold={MATCH_THRESHOLD})"
    )

    if best_score >= MATCH_THRESHOLD:
        return True, best_name, best_member_id, best_score
    return False, "", "", best_score


def capture_video_sequence(duration=CAPTURE_DURATION, fps=CAPTURE_FPS):
    if not camera_lock.acquire(timeout=5):
        print("[FaceService] Camera is busy (another capture in progress)")
        return []

    try:
        cap = cv2.VideoCapture(0, cv2.CAP_DSHOW)
        if not cap.isOpened():
            print("[FaceService] Failed to open camera")
            return []

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


@app.route("/recognize", methods=["POST"])
def recognize():
    image_bytes = request.data
    user_id = normalize_id(request.args.get("user_id"))
    if not user_id:
        return jsonify({"error": "user_id is required"}), 400
    if not image_bytes:
        return jsonify({"error": "no image data received"}), 400

    npimg = np.frombuffer(image_bytes, np.uint8)
    frame = cv2.imdecode(npimg, cv2.IMREAD_COLOR)
    if frame is None:
        return jsonify({"error": "could not decode image"}), 400

    match, user, member_id, score = recognize_face(frame, user_id)
    return jsonify({"match": match, "user": user, "user_id": user_id, "member_id": member_id, "score": score})


@app.route("/recognize-url", methods=["POST"])
def recognize_url():
    try:
        data = request.get_json(silent=True)
        if not data:
            return jsonify({"error": "request body must be JSON"}), 400

        user_id = normalize_id(data.get("user_id"))
        image_url = (data.get("image_url") or data.get("url") or "").strip()
        if not user_id:
            return jsonify({"error": "user_id is required"}), 400
        if not image_url:
            return jsonify({"error": "image_url is required"}), 400

        try:
            frame = load_image_from_url(image_url)
        except Exception as e:
            return jsonify({"error": f"failed to download image from URL: {str(e)}"}), 400

        match, user, member_id, score = recognize_face(frame, user_id)
        return jsonify({"match": match, "user": user, "user_id": user_id, "member_id": member_id, "score": score})

    except Exception as e:
        traceback.print_exc()
        return jsonify({"error": f"internal error: {str(e)}"}), 500


@app.route("/capture-and-recognize", methods=["POST"])
def capture_and_recognize():
    try:
        data = request.get_json(silent=True) or {}
        user_id = normalize_id(data.get("user_id"))
        candidates = data.get("candidates") or []

        frames = capture_video_sequence()
        if len(frames) < 5:
            return jsonify({"error": f"not enough frames captured ({len(frames)}), camera may be busy"}), 500

        alive, spoof_results = anti_spoof.run(frames)

        if not alive:
            print("[FaceService] SPOOF DETECTED - rejecting")
            mid_frame = frames[len(frames) // 2]
            _, buf = cv2.imencode(".jpg", mid_frame)
            frame_b64 = base64.b64encode(buf.tobytes()).decode("ascii")
            return jsonify({
                "match": False,
                "user": "",
                "user_id": user_id,
                "member_id": "",
                "score": -1.0,
                "spoof": True,
                "anti_spoof": spoof_results,
                "frame": frame_b64,
            })

        best_frame = frames[len(frames) // 2]
        match = False
        user = ""
        member_id = ""
        score = -1.0
        if candidates:
            match, user, member_id, score = recognize_face_candidates(best_frame, candidates)
        elif user_id:
            match, user, member_id, score = recognize_face(best_frame, user_id)

        _, buf = cv2.imencode(".jpg", best_frame)
        frame_b64 = base64.b64encode(buf.tobytes()).decode("ascii")

        print(f"[FaceService] capture-and-recognize -> user_id={user_id!r} alive=True match={match} user={user!r}")
        return jsonify({
            "match": match,
            "user": user,
            "user_id": user_id,
            "member_id": member_id,
            "score": score,
            "spoof": False,
            "anti_spoof": spoof_results,
            "frame": frame_b64,
        })

    except Exception as e:
        traceback.print_exc()
        return jsonify({"error": f"internal error: {str(e)}"}), 500


@app.route("/enroll", methods=["POST"])
def enroll():
    try:
        data = request.get_json(silent=True)
        if not data:
            return jsonify({"error": "request body must be JSON"}), 400

        user_id = normalize_id(data.get("user_id"))
        member_id = normalize_id(data.get("member_id"))
        name = (data.get("name") or "").strip()
        image_b64 = data.get("image") or ""

        if not image_b64:
            return jsonify({"error": "image is required (base64-encoded)"}), 400
        try:
            image_bytes = base64.b64decode(image_b64)
        except Exception:
            return jsonify({"error": "image is not valid base64"}), 400

        npimg = np.frombuffer(image_bytes, np.uint8)
        frame = cv2.imdecode(npimg, cv2.IMREAD_COLOR)
        if frame is None:
            return jsonify({"error": "could not decode image; unsupported format or corrupt data"}), 400

        result, error = enroll_frame(user_id, member_id, name, frame)
        if error:
            message, status = error
            return jsonify({"error": message}), status

        print(f"[FaceService] Enrolled: user={user_id!r} member={member_id!r} name={name!r}")
        return jsonify({
            "enrolled": name,
            "user_id": user_id,
            "member_id": member_id,
            "overwrite": result["overwrite"],
            "total_known": result["total_known"],
        }), 200

    except Exception as e:
        traceback.print_exc()
        return jsonify({"error": f"internal error: {str(e)}"}), 500


@app.route("/enroll-url", methods=["POST"])
def enroll_url():
    try:
        data = request.get_json(silent=True)
        if not data:
            return jsonify({"error": "request body must be JSON"}), 400

        user_id = normalize_id(data.get("user_id"))
        member_id = normalize_id(data.get("member_id"))
        name = (data.get("name") or "").strip()
        image_url = (data.get("image_url") or data.get("url") or "").strip()

        if not image_url:
            return jsonify({"error": "image_url is required"}), 400

        try:
            frame = load_image_from_url(image_url)
        except Exception as e:
            return jsonify({"error": f"failed to download image from URL: {str(e)}"}), 400

        result, error = enroll_frame(user_id, member_id, name, frame)
        if error:
            message, status = error
            return jsonify({"error": message}), status

        print(f"[FaceService] Enrolled from URL: user={user_id!r} member={member_id!r} name={name!r}")
        return jsonify({
            "enrolled": name,
            "user_id": user_id,
            "member_id": member_id,
            "overwrite": result["overwrite"],
            "total_known": result["total_known"],
        }), 200

    except Exception as e:
        traceback.print_exc()
        return jsonify({"error": f"internal error: {str(e)}"}), 500


@app.route("/embed-url", methods=["POST"])
def embed_url():
    try:
        data = request.get_json(silent=True)
        if not data:
            return jsonify({"error": "request body must be JSON"}), 400

        user_id = normalize_id(data.get("user_id"))
        member_id = normalize_id(data.get("member_id"))
        name = (data.get("name") or "").strip()
        image_url = (data.get("image_url") or data.get("url") or "").strip()

        if not user_id:
            return jsonify({"error": "user_id is required"}), 400
        if not member_id:
            return jsonify({"error": "member_id is required"}), 400
        if not name:
            return jsonify({"error": "name is required"}), 400
        if not _NAME_RE.match(name):
            return jsonify({"error": "name may only contain letters, digits, spaces, hyphens and underscores (max 64 chars)"}), 400
        if not image_url:
            return jsonify({"error": "image_url is required"}), 400

        try:
            frame = load_image_from_url(image_url)
        except Exception as e:
            return jsonify({"error": f"failed to download image from URL: {str(e)}"}), 400

        embedding, error = validate_single_face_and_embed(frame)
        if error:
            message, status = error
            return jsonify({"error": message}), status

        return jsonify({
            "user_id": user_id,
            "member_id": member_id,
            "name": name,
            "embedding": embedding_to_list(embedding),
        }), 200

    except Exception as e:
        traceback.print_exc()
        return jsonify({"error": f"internal error: {str(e)}"}), 500


@app.route("/faces", methods=["GET"])
def list_faces():
    user_id = normalize_id(request.args.get("user_id"))
    with faces_lock:
        if user_id:
            faces = known_faces.get(user_id, {})
            items = [
                {"member_id": record.get("member_id", member_id), "name": record.get("name", "")}
                for member_id, record in faces.items()
            ]
            count = len(items)
        else:
            items = {
                uid: [
                    {"member_id": record.get("member_id", member_id), "name": record.get("name", "")}
                    for member_id, record in faces.items()
                ]
                for uid, faces in known_faces.items()
            }
            count = sum(len(faces) for faces in items.values())

    return jsonify({"faces": items, "count": count})


@app.route("/faces/<user_id>/<member_id>", methods=["DELETE"])
def delete_face(user_id, member_id):
    user_id = normalize_id(user_id)
    member_id = normalize_id(member_id)
    with faces_lock:
        user_faces = known_faces.get(user_id, {})
        if member_id not in user_faces:
            return jsonify({"error": f"face user_id={user_id!r} member_id={member_id!r} not found"}), 404
        del user_faces[member_id]
        if not user_faces:
            known_faces.pop(user_id, None)
        total_known = len(known_faces.get(user_id, {}))

    save_embeddings()
    print(f"[FaceService] Deleted face: user={user_id!r} member={member_id!r}")
    return jsonify({"deleted": member_id, "user_id": user_id, "total_known": total_known})


@app.route("/health", methods=["GET"])
def health():
    with faces_lock:
        n = sum(len(faces) for faces in known_faces.values())
        users = {uid: len(faces) for uid, faces in known_faces.items()}
    return jsonify({
        "status": "ok",
        "model": "FaceNet (InceptionResnetV1 + VGGFace2)",
        "anti_spoof": "blink + head_movement + texture (2/3 required)",
        "device": str(device),
        "known_faces": n,
        "known_users": users,
    })


if __name__ == "__main__":
    load_known_faces()
    app.run(host="0.0.0.0", port=5000)
