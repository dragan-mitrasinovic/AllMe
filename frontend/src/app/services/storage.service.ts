import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { GetFolderContentsResponse } from '../models/search.model';
import { environment } from '../../environments/environment';

@Injectable({
  providedIn: 'root'
})
export class StorageService {
  private readonly http = inject(HttpClient);
  private readonly apiUrl = environment.apiUrl;

  getFolderContents(shareUrl: string, sessionId: string, provider: string): Observable<GetFolderContentsResponse> {
    const params = new HttpParams()
      .set('share_url', shareUrl)
      .set('session_id', sessionId)
      .set('provider', provider);

    return this.http.get<GetFolderContentsResponse>(`${this.apiUrl}/storage/folder-contents`, { params });
  }
}