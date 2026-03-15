import sys
import cv2

cap = cv2.VideoCapture(0)
if not cap.isOpened():
    sys.exit(1)

ret, frame = cap.read()
cap.release()

if not ret:
    sys.exit(1)

_, buf = cv2.imencode(".jpg", frame)
sys.stdout.buffer.write(buf.tobytes())
