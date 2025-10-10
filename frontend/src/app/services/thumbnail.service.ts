import { Injectable } from '@angular/core';
import { AuthService } from './auth.service';
import { environment } from '../../environments/environment';

@Injectable({
  providedIn: 'root'
})
export class ThumbnailService {
  private readonly apiUrl = environment.apiUrl;

  constructor(private authService: AuthService) {}

  getProxiedThumbnailUrl(originalUrl: string, provider: string): string {
    if (!originalUrl || !provider) {
      return '';
    }

    // Don't proxy blob URLs (for uploaded files)
    if (originalUrl.startsWith('blob:')) {
      return originalUrl;
    }

    // Check if this URL needs proxying (Google Drive or OneDrive thumbnail URLs)
    const needsProxy = originalUrl.includes('googleapis.com') || 
                      originalUrl.includes('graph.microsoft.com') ||
                      originalUrl.includes('thumbnails/');

    if (!needsProxy) {
      return originalUrl;
    }

    const sessionId = this.authService.getSessionId();
    if (!sessionId) {
      return originalUrl; // Fallback to original URL if no session
    }

    // Create proxied URL with provider parameter
    const encodedUrl = encodeURIComponent(originalUrl);
    return `${this.apiUrl}/thumbnail?session_id=${sessionId}&url=${encodedUrl}&provider=${provider}`;
  }
}