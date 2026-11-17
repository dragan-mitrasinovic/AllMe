"""Face detection and matching. These functions run in worker processes (see WORKERS in main.py), so they
only take and return plain, picklable values."""
import base64
from io import BytesIO
from typing import Optional, Tuple

import face_recognition
import numpy as np
from PIL import Image

# Faces at this distance or closer count as a match
MATCH_THRESHOLD = 0.7


def load_image(image_base64: str) -> np.ndarray:
    image = Image.open(BytesIO(base64.b64decode(image_base64)))
    if image.mode != "RGB":
        image = image.convert("RGB")
    return np.array(image)


def encode_reference_face(image_base64: str) -> Tuple[str, Optional[np.ndarray]]:
    """Returns ("ok", encoding) or (reason, None) with reason "invalid_image", "no_face",
    "multiple_faces" or "no_encoding"."""
    try:
        image_array = load_image(image_base64)
    except Exception:
        return "invalid_image", None

    face_locations = face_recognition.face_locations(image_array)
    if len(face_locations) == 0:
        return "no_face", None
    if len(face_locations) > 1:
        return "multiple_faces", None

    face_encodings = face_recognition.face_encodings(image_array, face_locations)
    if len(face_encodings) == 0:
        return "no_encoding", None
    return "ok", face_encodings[0]


def best_match_distance(image_base64: str, base_encoding: np.ndarray) -> Optional[float]:
    """Distance of the face in the image closest to base_encoding, or None if no face matches."""
    image_array = load_image(image_base64)

    face_locations = face_recognition.face_locations(image_array)
    if len(face_locations) == 0:
        return None

    face_encodings = face_recognition.face_encodings(image_array, face_locations)
    if len(face_encodings) == 0:
        return None

    best = float(np.min(face_recognition.face_distance(face_encodings, base_encoding)))
    return best if best <= MATCH_THRESHOLD else None
