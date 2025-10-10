import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { CloudItem } from '../models/auth.model';
import { environment } from '../../environments/environment';

@Injectable({
  providedIn: 'root'
})
export class DownloadService {
  private readonly http = inject(HttpClient);
  private readonly apiUrl = environment.apiUrl;

  downloadZip(files: CloudItem[], sessionId: string, provider: string): Observable<Blob> {
    return this.http.post(`${this.apiUrl}/downloads/zip`, { 
      files,
      session_id: sessionId,
      provider: provider
    }, {
      responseType: 'blob'
    });
  }

  triggerDownload(blob: Blob, filename: string): void {
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  }
}