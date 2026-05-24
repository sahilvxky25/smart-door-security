## AWS:

1. Add config flag: ENABLE_LOCAL_DOOR_PEER=false
2. Only start DoorPeer when that flag is true
3. Add Linux-safe SoundService implementation
4. Add backend Dockerfile
5. Update docker-compose.yml with backend + dev-network
6. Keep door camera/mic process outside Docker
7. Later replace Mosquitto with AWS IoT Core for production

## To make it internship-ready, tighten it in these areas:

1. Security hygiene: remove secrets from Git history, keep .env ignored, rotate leaked keys, avoid committing firebase-adminsdk.json.
2. Architecture clarity: document the system diagram: Flutter app, backend, MQTT broker, Postgres, ESP32, face service, WebRTC signaling.
3. Deployment story: add Docker for backend, compose for local infra, and a short AWS deployment plan.
4. Reliability: add basic health checks, config validation, and cleaner startup errors when DB/MQTT/Cloudinary are missing.
5. Code quality: fix platform-specific backend code like Windows-only SoundService before Linux/Docker deployment.
6. Tests: add at least a few backend unit tests for auth, JWT, event flow, and service logic.
7. Demo polish: record a 2-3 minute demo showing door event -> MQTT -> backend -> app notification/WebRTC flow
