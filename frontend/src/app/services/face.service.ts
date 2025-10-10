import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, interval, switchMap, takeWhile, map, startWith } from 'rxjs';
import { FaceRegisterResponse, CompareFolderRequest, CompareFolderResponse, JobStatusResponse } from '../models/search.model';
import { environment } from '../../environments/environment';

@Injectable({
  providedIn: 'root'
})
export class FaceService {
  private readonly http = inject(HttpClient);
  private readonly apiUrl = environment.apiUrl;

  registerBaseFace(sessionId: string, imageFile: File): Observable<FaceRegisterResponse> {
    const formData = new FormData();
    formData.append('image', imageFile);
    formData.append('session_id', sessionId);
    return this.http.post<FaceRegisterResponse>(`${this.apiUrl}/face/register-base`, formData);
  }

  compareFolder(sessionId: string, folderLink: string, provider: string, recursive: boolean = false): Observable<CompareFolderResponse> {
    const request: CompareFolderRequest = {
      session_id: sessionId,
      folder_link: folderLink,
      provider: provider,
      recursive: recursive
    };
    return this.http.post<CompareFolderResponse>(`${this.apiUrl}/face/compare-folder`, request);
  }

  getJobStatus(jobId: string): Observable<JobStatusResponse> {
    return this.http.get<JobStatusResponse>(`${this.apiUrl}/face/job-status/${jobId}`);
  }

  pollJobStatus(jobId: string, pollIntervalMs: number = 1500): Observable<JobStatusResponse> {
    return interval(pollIntervalMs).pipe(
      startWith(0),
      switchMap(() => this.getJobStatus(jobId)),
      takeWhile((status) => status.status === 'processing', true)
    );
  }

  clearReferenceImage(sessionId: string): Observable<any> {
    return this.http.delete(`${this.apiUrl}/face/clear-reference/${sessionId}`);
  }
}
