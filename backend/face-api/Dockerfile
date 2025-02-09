FROM animcogn/face_recognition:latest

COPY main.py main.py
COPY requirements.txt requirements.txt

RUN pip install -r ./requirements.txt

EXPOSE ${FACE_API_PORT}

ENTRYPOINT ["python", "main.py"]