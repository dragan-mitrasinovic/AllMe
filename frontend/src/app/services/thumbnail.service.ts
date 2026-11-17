import { Injectable } from '@angular/core';
import { environment } from '../../environments/environment';

@Injectable({
  providedIn: 'root'
})
export class ThumbnailService {
  private readonly apiUrl = environment.apiUrl;

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

    // The session cookie authenticates the proxied request
    const encodedUrl = encodeURIComponent(originalUrl);
    return `${this.apiUrl}/thumbnail?url=${encodedUrl}&provider=${provider}`;
  }
}