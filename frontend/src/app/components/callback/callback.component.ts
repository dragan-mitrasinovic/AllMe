import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { AuthService } from '../../services/auth.service';
import { StorageService } from '../../services/storage.service';
import { FaceService } from '../../services/face.service';

@Component({
  selector: 'app-callback',
  templateUrl: './callback.component.html',
  styleUrl: './callback.component.css',
  imports: [
    CommonModule,
    MatProgressSpinnerModule,
    MatCardModule,
    MatIconModule
  ]
})
export class CallbackComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly authService = inject(AuthService);
  private readonly storageService = inject(StorageService);
  private readonly faceService = inject(FaceService);

  errorMessage: string = '';
  isProcessing: boolean = true;

  ngOnInit(): void {
    this.route.queryParams.subscribe(params => {
      const error = params['error'];
      const success = params['success'];
      const provider = params['provider'];

      if (error) {
        const message = params['message'] || params['error_description'] || 'Authentication failed';
        this.errorMessage = message;
        this.isProcessing = false;
        setTimeout(() => this.router.navigate(['/']), 3000);
        return;
      }

      if (success && provider) {
        this.handleSuccessfulAuth(provider);
      } else {
        this.errorMessage = 'Invalid callback parameters';
        this.isProcessing = false;
        setTimeout(() => this.router.navigate(['/']), 3000);
      }
    });
  }

  private handleSuccessfulAuth(provider: string): void {
    const folderLink = sessionStorage.getItem('folderLink');
    const sessionId = this.authService.getSessionId();

    if (!folderLink || !sessionId) {
      this.errorMessage = 'Session expired. Please start over.';
      this.isProcessing = false;
      setTimeout(() => this.router.navigate(['/']), 2000);
      return;
    }

    // Clear any existing reference image for this session (but keep the auth token)
    this.faceService.clearReferenceImage(sessionId).subscribe({
      next: () => {
        // Reference image cleared, now get folder contents
        this.fetchFolderContents(folderLink, sessionId, provider);
      },
      error: () => {
        // Even if clearing fails, proceed to get folder contents
        // (reference image might not exist, which is fine)
        this.fetchFolderContents(folderLink, sessionId, provider);
      }
    });
  }

  private fetchFolderContents(folderLink: string, sessionId: string, provider: string): void {
    // Get folder contents with session ID and provider
    this.storageService.getFolderContents(folderLink, sessionId, provider).subscribe({
      next: (response) => {
        this.isProcessing = false;
        
        // Navigate to search page with folder data (keep folderLink for face matching)
        this.router.navigate(['/search'], {
          state: { 
            contents: response.contents,
            folder: response.folder,
            provider: provider,
            folderLink: folderLink
          }
        });
      },
      error: (error) => {
        this.isProcessing = false;
        this.errorMessage = error.message || 'Failed to access folder. Please try again.';
        setTimeout(() => this.router.navigate(['/']), 3000);
      }
    });
  }
}
