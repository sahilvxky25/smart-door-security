import cv2
import numpy as np
import mediapipe as mp
import os

BaseOptions = mp.tasks.BaseOptions
FaceLandmarker = mp.tasks.vision.FaceLandmarker
FaceLandmarkerOptions = mp.tasks.vision.FaceLandmarkerOptions
RunningMode = mp.tasks.vision.RunningMode

# Path to the downloaded model (relative to working directory)
MODEL_PATH = os.path.join(os.path.dirname(__file__), "face_landmarker.task")


class AntiSpoof:
    """Liveness detection using three passive checks:
    1. Blink detection (EAR drop across frames)
    2. Head movement (nose tip displacement across frames)
    3. Texture analysis (LBP histogram variance on face region)

    Requires at least 2/3 checks to pass for a "live" verdict.
    """

    # MediaPipe Face Mesh landmark indices (478 landmarks)
    # Left eye:  p0=362, p1=385, p2=387, p3=263, p4=373, p5=380
    # Right eye: p0=33,  p1=160, p2=158, p3=133, p4=153, p5=144
    LEFT_EYE = [362, 385, 387, 263, 373, 380]
    RIGHT_EYE = [33, 160, 158, 133, 153, 144]
    NOSE_TIP = 1

    EAR_THRESHOLD = 0.22  # eye aspect ratio below this = eye closed
    HEAD_MOVE_THRESHOLD = 8.0  # minimum total nose movement in pixels
    LBP_VARIANCE_THRESHOLD = 0.00005  # minimum LBP histogram variance

    def __init__(self):
        options = FaceLandmarkerOptions(
            base_options=BaseOptions(model_asset_path=MODEL_PATH),
            running_mode=RunningMode.IMAGE,
            num_faces=1,
        )
        self.landmarker = FaceLandmarker.create_from_options(options)

    def _get_landmarks(self, frame):
        """Run FaceLandmarker on a BGR frame.
        Returns list of 478 NormalizedLandmark or None."""
        rgb = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
        mp_image = mp.Image(image_format=mp.ImageFormat.SRGB, data=rgb)
        result = self.landmarker.detect(mp_image)
        if not result.face_landmarks:
            return None
        return result.face_landmarks[0]

    # --------------------------------------------------
    # EAR (Eye Aspect Ratio)
    # --------------------------------------------------
    def _ear(self, landmarks, eye_indices, w, h):
        pts = np.array(
            [(landmarks[i].x * w, landmarks[i].y * h) for i in eye_indices]
        )
        v1 = np.linalg.norm(pts[1] - pts[5])
        v2 = np.linalg.norm(pts[2] - pts[4])
        h1 = np.linalg.norm(pts[0] - pts[3])
        if h1 == 0:
            return 0.0
        return (v1 + v2) / (2.0 * h1)

    # --------------------------------------------------
    # 1. BLINK DETECTION
    # --------------------------------------------------
    def check_blink(self, frames):
        """Detect at least one natural blink (EAR dip + recovery)."""
        ear_values = []

        for frame in frames:
            lm = self._get_landmarks(frame)
            if lm is None:
                continue
            h, w = frame.shape[:2]
            left = self._ear(lm, self.LEFT_EYE, w, h)
            right = self._ear(lm, self.RIGHT_EYE, w, h)
            ear_values.append((left + right) / 2.0)

        if len(ear_values) < 5:
            return False, "not enough face landmarks detected"

        # A blink = EAR drops below threshold then rises back
        below = False
        for ear in ear_values:
            if ear < self.EAR_THRESHOLD:
                below = True
            elif below and ear >= self.EAR_THRESHOLD:
                return True, f"blink detected (min EAR={min(ear_values):.3f})"

        return False, f"no blink (EAR range {min(ear_values):.3f}-{max(ear_values):.3f})"

    # --------------------------------------------------
    # 2. HEAD MOVEMENT
    # --------------------------------------------------
    def check_head_movement(self, frames):
        """Check natural head motion — a photo/screen stays fixed."""
        nose_positions = []

        for frame in frames:
            lm = self._get_landmarks(frame)
            if lm is None:
                continue
            h, w = frame.shape[:2]
            nose = lm[self.NOSE_TIP]
            nose_positions.append(np.array([nose.x * w, nose.y * h]))

        if len(nose_positions) < 5:
            return False, "not enough face landmarks detected"

        total_movement = sum(
            np.linalg.norm(nose_positions[i] - nose_positions[i - 1])
            for i in range(1, len(nose_positions))
        )

        passed = total_movement > self.HEAD_MOVE_THRESHOLD
        return passed, f"nose movement={total_movement:.1f}px (threshold={self.HEAD_MOVE_THRESHOLD})"

    # --------------------------------------------------
    # 3. TEXTURE ANALYSIS (LBP)
    # --------------------------------------------------
    def check_texture(self, frame):
        """LBP texture analysis — printed photos and screens have
        smoother, more uniform texture than real skin."""
        lm = self._get_landmarks(frame)
        if lm is None:
            return False, "no face detected for texture check"

        h, w = frame.shape[:2]

        # Bounding box from face landmarks
        xs = [l.x * w for l in lm]
        ys = [l.y * h for l in lm]
        x1 = max(0, int(min(xs)))
        y1 = max(0, int(min(ys)))
        x2 = min(w, int(max(xs)))
        y2 = min(h, int(max(ys)))

        face_crop = frame[y1:y2, x1:x2]
        if face_crop.size == 0:
            return False, "face crop is empty"

        gray = cv2.cvtColor(face_crop, cv2.COLOR_BGR2GRAY)
        lbp = self._compute_lbp(gray)

        hist, _ = np.histogram(lbp.ravel(), bins=256, range=(0, 256))
        hist = hist.astype(np.float64) / (hist.sum() + 1e-7)
        variance = np.var(hist)

        passed = variance > self.LBP_VARIANCE_THRESHOLD
        return passed, f"LBP variance={variance:.7f} (threshold={self.LBP_VARIANCE_THRESHOLD})"

    def _compute_lbp(self, gray):
        """Vectorized 3x3 LBP — no Python loops over pixels."""
        g = gray.astype(np.int16)
        padded = np.pad(g, 1, mode="edge")
        r, c = g.shape
        center = padded[1 : r + 1, 1 : c + 1]

        lbp = np.zeros((r, c), dtype=np.uint8)
        lbp |= (padded[0:r, 0:c] >= center).astype(np.uint8) << 7
        lbp |= (padded[0:r, 1 : c + 1] >= center).astype(np.uint8) << 6
        lbp |= (padded[0:r, 2 : c + 2] >= center).astype(np.uint8) << 5
        lbp |= (padded[1 : r + 1, 2 : c + 2] >= center).astype(np.uint8) << 4
        lbp |= (padded[2 : r + 2, 2 : c + 2] >= center).astype(np.uint8) << 3
        lbp |= (padded[2 : r + 2, 1 : c + 1] >= center).astype(np.uint8) << 2
        lbp |= (padded[2 : r + 2, 0:c] >= center).astype(np.uint8) << 1
        lbp |= (padded[1 : r + 1, 0:c] >= center).astype(np.uint8) << 0
        return lbp

    # --------------------------------------------------
    # RUN ALL CHECKS
    # --------------------------------------------------
    def run(self, frames):
        """Run all three anti-spoof checks on a frame sequence.
        Returns (alive: bool, results: dict).
        Requires at least 2/3 checks to pass."""

        mid_frame = frames[len(frames) // 2]

        blink_ok, blink_detail = self.check_blink(frames)
        head_ok, head_detail = self.check_head_movement(frames)
        texture_ok, texture_detail = self.check_texture(mid_frame)

        checks = {
            "blink": {"passed": bool(blink_ok), "detail": blink_detail},
            "head_movement": {"passed": bool(head_ok), "detail": head_detail},
            "texture": {"passed": bool(texture_ok), "detail": texture_detail},
        }

        passed_count = sum(1 for v in checks.values() if v["passed"])
        alive = passed_count >= 2

        print(f"[AntiSpoof] blink   = {'PASS' if blink_ok else 'FAIL'} | {blink_detail}")
        print(f"[AntiSpoof] head    = {'PASS' if head_ok else 'FAIL'} | {head_detail}")
        print(f"[AntiSpoof] texture = {'PASS' if texture_ok else 'FAIL'} | {texture_detail}")
        print(f"[AntiSpoof] verdict = {'ALIVE' if alive else 'SPOOF'} ({passed_count}/3 passed)")

        return alive, checks
