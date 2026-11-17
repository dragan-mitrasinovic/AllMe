import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../environments/environment';

export interface SessionValidationResponse {
  valid: boolean;
  requires_auth: boolean;
  provider?: string;
}

// The session lives in an HttpOnly cookie issued by the backend at login; the browser sends it with every
// /api request, so the app never sees or handles a session ID itself.
@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private readonly apiUrl = environment.apiUrl;

  constructor(private http: HttpClient) {}

  initiateOAuth(provider: string): string {
    return `${this.apiUrl}/auth/${provider}/login`;
  }

  validateSession(provider: string): Observable<SessionValidationResponse> {
    const params = new HttpParams().set('provider', provider);
    return this.http.get<SessionValidationResponse>(`${this.apiUrl}/auth/validate-session`, { params });
  }

  clearSession(): void {
    sessionStorage.clear();
  }

  clearFolderContext(): void {
    sessionStorage.removeItem('folderLink');
    sessionStorage.removeItem('provider');
  }

  signOutProvider(provider: string): Observable<any> {
    const params = new HttpParams().set('provider', provider);
    return this.http.post(`${this.apiUrl}/auth/signout`, null, { params });
  }
}
