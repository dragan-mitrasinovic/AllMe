import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../environments/environment';

export interface SessionValidationResponse {
  valid: boolean;
  requires_auth: boolean;
  provider?: string;
}

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private readonly apiUrl = environment.apiUrl;
  private sessionId: string | null = null;

  constructor(private http: HttpClient) {
    this.sessionId = sessionStorage.getItem('sessionId');
  }

  initiateOAuth(provider: string): string {
    const sessionId = this.getOrCreateSessionId();
    return `${this.apiUrl}/auth/${provider}/login?session_id=${sessionId}`;
  }

  validateSession(provider: string): Observable<SessionValidationResponse> {
    const sessionId = this.getOrCreateSessionId();
    const params = new HttpParams()
      .set('session_id', sessionId)
      .set('provider', provider);
    
    return this.http.get<SessionValidationResponse>(`${this.apiUrl}/auth/validate-session`, { params });
  }

  getSessionId(): string | null {
    return this.sessionId;
  }

  getOrCreateSessionId(): string {
    if (!this.sessionId) {
      this.sessionId = this.generateSessionId();
      sessionStorage.setItem('sessionId', this.sessionId);
    }
    return this.sessionId;
  }

  private generateSessionId(): string {
    return 'session_' + Math.random().toString(36).substring(2) + Date.now().toString(36);
  }

  clearSession(): void {
    this.sessionId = null;
    sessionStorage.clear();
  }

  clearFolderContext(): void {
    // Clear only folder-related session storage, keep sessionId and tokens
    sessionStorage.removeItem('folderLink');
    sessionStorage.removeItem('provider');
  }

  signOutProvider(provider: string): Observable<any> {
    const sessionId = this.getSessionId();
    if (!sessionId) {
      return new Observable(observer => {
        observer.next({ success: true });
        observer.complete();
      });
    }

    const params = new HttpParams()
      .set('session_id', sessionId)
      .set('provider', provider);
    
    return this.http.post(`${this.apiUrl}/auth/signout`, null, { params });
  }
}