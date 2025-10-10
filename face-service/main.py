from fastapi import FastAPI, HTTPException, BackgroundTasks
from pydantic import BaseModel
import logging
from typing import Dict, Optional, List
import numpy as np
import face_recognition
import base64
from io import BytesIO
from PIL import Image
import uuid
from datetime import datetime, timedelta
import asyncio
import threading

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class SessionData:
    def __init__(self, encoding: np.ndarray):
        self.encoding = encoding
        self.created_at = datetime.now()
        self.last_accessed = datetime.now()

class SessionStore:
    def __init__(self):
        self.sessions: Dict[str, SessionData] = {}
        self.session_ttl = timedelta(hours=24)
        self._start_cleanup_task()
    
    def store(self, session_id: str, encoding: np.ndarray) -> None:
        self.sessions[session_id] = SessionData(encoding)
    
    def retrieve(self, session_id: str) -> Optional[np.ndarray]:
        session_data = self.sessions.get(session_id)
        if session_data:
            session_data.last_accessed = datetime.now()
            return session_data.encoding
        return None
    
    def delete(self, session_id: str) -> bool:
        if session_id in self.sessions:
            del self.sessions[session_id]
            return True
        return False
    
    def cleanup_expired_sessions(self) -> int:
        """Remove sessions that have exceeded the TTL"""
        now = datetime.now()
        expired_sessions = []
        
        for session_id, session_data in self.sessions.items():
            if now - session_data.last_accessed > self.session_ttl:
                expired_sessions.append(session_id)
        
        for session_id in expired_sessions:
            del self.sessions[session_id]
        
        return len(expired_sessions)
    
    def _start_cleanup_task(self):
        """Start the background cleanup task"""
        def cleanup_worker():
            while True:
                try:
                    self.cleanup_expired_sessions()
                except Exception as e:
                    logger.error(f"Error during session cleanup: {e}")
                # Sleep for 1 hour
                threading.Event().wait(3600)
        
        cleanup_thread = threading.Thread(target=cleanup_worker, daemon=True)
        cleanup_thread.start()

class MatchResult:
    def __init__(self, index: int, distance: float):
        self.index = index
        self.distance = distance

class JobStatus:
    def __init__(self, job_id: str, total_images: int):
        self.job_id = job_id
        self.status = "processing"
        self.progress = 0
        self.current_image = 0
        self.total_images = total_images
        self.matches_found = 0
        self.matches: List[MatchResult] = []
        self.message = "Starting processing..."
        self.error: Optional[str] = None
        self.created_at = datetime.now()

class JobStore:
    def __init__(self):
        self.jobs: Dict[str, JobStatus] = {}
    
    def create_job(self, total_images: int) -> str:
        job_id = str(uuid.uuid4())
        self.jobs[job_id] = JobStatus(job_id, total_images)
        return job_id
    
    def get_job(self, job_id: str) -> Optional[JobStatus]:
        return self.jobs.get(job_id)
    
    def update_progress(self, job_id: str, current: int, matches_found: int):
        job = self.jobs.get(job_id)
        if job:
            job.current_image = current
            job.matches_found = matches_found
            job.progress = int((current / job.total_images) * 100) if job.total_images > 0 else 0
            job.message = f"Processing image {current} of {job.total_images}"
    
    def complete_job(self, job_id: str, matches: List[MatchResult]):
        job = self.jobs.get(job_id)
        if job:
            job.status = "completed"
            job.progress = 100
            job.matches = matches
            job.matches_found = len(matches)
            job.message = f"Completed! Found {len(matches)} matches"
    
    def fail_job(self, job_id: str, error: str):
        job = self.jobs.get(job_id)
        if job:
            job.status = "failed"
            job.error = error
            job.message = f"Failed: {error}"
    
    def delete_job(self, job_id: str) -> bool:
        if job_id in self.jobs:
            del self.jobs[job_id]
            return True
        return False

session_store = SessionStore()
job_store = JobStore()

app = FastAPI(
    title="Face Recognition Service",
    description="Microservice for face detection and comparison",
    version="1.0.0"
)

@app.get("/health")
async def health_check():
    return {
        "status": "healthy",
        "service": "face-recognition",
        "version": "1.0.0",
        "active_sessions": len(session_store.sessions),
        "timestamp": datetime.now().isoformat()
    }

# Request/Response Models
class RegisterRequest(BaseModel):
    session_id: str
    image: str  # base64 encoded image

class RegisterResponse(BaseModel):
    success: bool

class ErrorResponse(BaseModel):
    error: str

class CompareBatchRequest(BaseModel):
    session_id: str
    images: List[str]  # list of base64 encoded images

class CompareBatchResponse(BaseModel):
    job_id: str
    status: str

class MatchResultModel(BaseModel):
    index: int
    distance: float

class JobStatusResponse(BaseModel):
    job_id: str
    status: str
    progress: int
    current_image: int
    total_images: int
    matches_found: int
    message: str
    matches: Optional[List[MatchResultModel]] = None
    error: Optional[str] = None

@app.post("/face/register", response_model=RegisterResponse)
async def register_face(request: RegisterRequest):
    """Register a base face for a session"""
    try:
        # Decode base64 image
        try:
            image_data = base64.b64decode(request.image)
        except Exception as e:
            raise HTTPException(status_code=400, detail="Invalid image format")
        
        # Load image with PIL
        try:
            image = Image.open(BytesIO(image_data))
            if image.mode != 'RGB':
                image = image.convert('RGB')
            image_array = np.array(image)
        except Exception as e:
            raise HTTPException(status_code=400, detail="Invalid image format")
        
        # Detect faces in the image
        face_locations = face_recognition.face_locations(image_array)
        
        # Validate exactly one face is detected
        if len(face_locations) == 0:
            raise HTTPException(status_code=400, detail="No face detected in image")
        
        if len(face_locations) > 1:
            raise HTTPException(status_code=400, detail="Multiple faces detected, please use image with single face")
        
        # Extract face encoding
        face_encodings = face_recognition.face_encodings(image_array, face_locations)
        
        if len(face_encodings) == 0:
            raise HTTPException(status_code=500, detail="Failed to extract face encoding")
        
        face_encoding = face_encodings[0]
        
        session_store.store(request.session_id, face_encoding)
        return RegisterResponse(success=True)
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Unexpected error in register_face: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")

def process_batch_background(job_id: str, session_id: str, images: List[str]):
    """Background task to process images"""
    try:
        base_encoding = session_store.retrieve(session_id)
        if base_encoding is None:
            job_store.fail_job(job_id, "Session not found")
            return
        
        matches = []
        total_images = len(images)
        
        for idx, image_base64 in enumerate(images):
            try:
                image_data = base64.b64decode(image_base64)
                image = Image.open(BytesIO(image_data))
                if image.mode != 'RGB':
                    image = image.convert('RGB')
                image_array = np.array(image)
                
                face_locations = face_recognition.face_locations(image_array)
                
                if len(face_locations) > 0:
                    face_encodings = face_recognition.face_encodings(image_array, face_locations)
                    
                    # Compare all faces in the image and keep the best match
                    best_distance = float('inf')
                    
                    for face_encoding in face_encodings:
                        # Calculate face distance
                        distances = face_recognition.face_distance([base_encoding], face_encoding)
                        distance = distances[0]
                        
                        # Use 0.7 as the maximum threshold and track the best matching distance
                        if distance <= 0.7 and distance < best_distance:
                            best_distance = distance
                    
                    # If any face matched, add the image with the best distance
                    if best_distance <= 0.7:
                        matches.append(MatchResult(idx, float(best_distance)))
                
                job_store.update_progress(job_id, idx + 1, len(matches))
                        
            except Exception as e:
                logger.warning(f"Failed to process image at index {idx} for job {job_id}: {e}")
                continue
        
        job_store.complete_job(job_id, matches)
        
    except Exception as e:
        logger.error(f"Unexpected error in background processing for job {job_id}: {e}")
        job_store.fail_job(job_id, str(e))

@app.post("/face/compare-batch", response_model=CompareBatchResponse)
async def compare_batch(request: CompareBatchRequest, background_tasks: BackgroundTasks):
    """Start a batch comparison job"""
    try:
        base_encoding = session_store.retrieve(request.session_id)
        if base_encoding is None:
            raise HTTPException(status_code=404, detail="Session not found")
        
        job_id = job_store.create_job(len(request.images))
        
        background_tasks.add_task(process_batch_background, job_id, request.session_id, request.images)
        
        return CompareBatchResponse(
            job_id=job_id,
            status="processing"
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Unexpected error in compare_batch: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")

@app.get("/face/job-status/{job_id}", response_model=JobStatusResponse)
async def get_job_status(job_id: str):
    """Get the status of a comparison job"""
    try:
        job = job_store.get_job(job_id)
        
        if job is None:
            raise HTTPException(status_code=404, detail="Job not found")
        
        # Convert MatchResult objects to MatchResultModel for the response
        matches_data = None
        if job.status == "completed" and job.matches:
            matches_data = [MatchResultModel(index=m.index, distance=m.distance) for m in job.matches]
        
        return JobStatusResponse(
            job_id=job.job_id,
            status=job.status,
            progress=job.progress,
            current_image=job.current_image,
            total_images=job.total_images,
            matches_found=job.matches_found,
            message=job.message,
            matches=matches_data,
            error=job.error
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Unexpected error in get_job_status: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")

@app.delete("/face/session/{session_id}")
async def delete_session(session_id: str):
    try:
        success = session_store.delete(session_id)
        if success:
            return {"success": True}
        else:
            raise HTTPException(status_code=404, detail="Session not found")
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error during session cleanup for {session_id}: {e}")
        raise HTTPException(status_code=500, detail="Internal server error")
