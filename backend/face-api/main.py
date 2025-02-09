from flask import Flask, jsonify, request

import os
import face_recognition
import requests
import tempfile
import json

CHUNK_SIZE = 8 * 1024

def download_image(download_url: str, file_extension: str) -> str:
    response = requests.get(download_url, stream=True)
    response.raise_for_status()

    with tempfile.NamedTemporaryFile(suffix=file_extension, delete=False) as file:
        for chunk in response.iter_content(CHUNK_SIZE):
            file.write(chunk)

    return file.name

app = Flask(__name__)

@app.route('/find_matches', methods=['POST'])
def find_matches():
    user_image_file = request.files.get('user_image', None)
    target_images_json = request.form.get('target_images', None)

    if not user_image_file or not target_images_json:
        return jsonify({"error": "Invalid request"}), 400

    targets = json.loads(target_images_json)

    user_image = face_recognition.load_image_file(user_image_file)
    user_encodings = face_recognition.face_encodings(user_image)

    if not user_encodings:
        return jsonify({"error": "No face found in user image"}), 400

    user_encoding = user_encodings[0]
    results = []

    for target in targets['target_images']:
        name = target.get('name')
        url = target.get('url')

        extension = name.split('.')[-1]
        temp_path = download_image(url, extension)

        target_image = face_recognition.load_image_file(temp_path)
        target_encodings = face_recognition.face_encodings(target_image)
        os.remove(temp_path)

        if len(target_encodings) == 0:
            continue

        matches = face_recognition.compare_faces(
            target_encodings,
            user_encoding,
            tolerance=0.6
        )

        if any(matches):
            results.append({"name": name, "url": url})

    return jsonify({"results": results})


if __name__ == '__main__':
    port = os.getenv('FACE_API_PORT')
    if not port:
        raise RuntimeError("PORT environment variable not set")

    app.run(host='0.0.0.0', port=int(port))