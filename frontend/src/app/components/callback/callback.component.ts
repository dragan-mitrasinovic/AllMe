import { Component, OnInit, inject, ChangeDetectionStrategy } from '@angular/core';

import { ActivatedRoute, Router } from '@angular/router';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { StorageService } from '../../services/storage.service';
import { FaceService } from '../../services/face.service';

@Component({
  selector: 'app-callback',
  templateUrl: './callback.component.html',
  styleUrl: './callback.component.css',
  changeDetection: ChangeDetectionStrategy.Eager,
  imports: [
    MatProgressSpinnerModule,
    MatCardModule,
    MatIconModule
]
})
export class CallbackComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
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
        // Only known codes are shown; text from the URL is never displayed, so a crafted link can't put
        // arbitrary messages on the page.
        this.errorMessage = error === 'access_denied'
          ? 'Sign-in was cancelled.'
          : 'Sign-in failed. Please try again.';
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

    if (!folderLink) {
      this.errorMessage = 'Session expired. Please start over.';
      this.isProcessing = false;
      setTimeout(() => this.router.navigate(['/']), 2000);
      return;
    }

    // Clear any existing reference image for this session (but keep the auth token)
    this.faceService.clearReferenceImage().subscribe({
      next: () => {
        // Reference image cleared, now get folder contents
        this.fetchFolderContents(folderLink, provider);
      },
      error: () => {
        // Even if clearing fails, proceed to get folder contents
        // (reference image might not exist, which is fine)
        this.fetchFolderContents(folderLink, provider);
      }
    });
  }

  private fetchFolderContents(folderLink: string, provider: string): void {
    this.storageService.getFolderContents(folderLink, provider).subscribe({
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
