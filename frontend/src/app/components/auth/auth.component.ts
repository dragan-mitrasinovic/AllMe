import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { AuthService } from '../../services/auth.service';
import { StorageService } from '../../services/storage.service';
import { FaceService } from '../../services/face.service';

@Component({
  selector: 'app-auth',
  templateUrl: './auth.component.html',
  styleUrl: './auth.component.css',
  imports: [
    CommonModule, 
    FormsModule,
    MatCardModule,
    MatFormFieldModule,
    MatSelectModule,
    MatInputModule,
    MatButtonModule,
    MatIconModule,
    MatProgressSpinnerModule
  ]
})
export class AuthComponent {
  private readonly authService = inject(AuthService);
  private readonly storageService = inject(StorageService);
  private readonly faceService = inject(FaceService);
  private readonly router = inject(Router);

  selectedProvider: 'onedrive' | 'googledrive' = 'onedrive';
  folderLink: string = '';
  errorMessage: string = '';
  isLoading: boolean = false;
  showProcessingLoader: boolean = false;
  
  // Track authentication status for each provider
  providerAuthStatus: { [key: string]: boolean } = {
    'onedrive': false,
    'googledrive': false
  };

  constructor() {
    // Check authentication status on component init
    this.checkAuthStatus();
  }

  private checkAuthStatus(): void {
    const sessionId = this.authService.getSessionId();
    if (!sessionId) {
      return;
    }

    // Check OneDrive
    this.authService.validateSession('onedrive').subscribe({
      next: (validation) => {
        this.providerAuthStatus['onedrive'] = validation.valid && !validation.requires_auth;
      },
      error: () => {
        this.providerAuthStatus['onedrive'] = false;
      }
    });

    // Check Google Drive
    this.authService.validateSession('googledrive').subscribe({
      next: (validation) => {
        this.providerAuthStatus['googledrive'] = validation.valid && !validation.requires_auth;
      },
      error: () => {
        this.providerAuthStatus['googledrive'] = false;
      }
    });
  }

  isProviderAuthenticated(provider: string): boolean {
    return this.providerAuthStatus[provider] || false;
  }

  onSignOut(provider: 'onedrive' | 'googledrive', event: Event): void {
    event.stopPropagation();
    
    this.authService.signOutProvider(provider).subscribe({
      next: () => {
        this.providerAuthStatus[provider] = false;
        // If we sign out of the currently selected provider, clear the folder link
        if (this.selectedProvider === provider) {
          this.folderLink = '';
        }
      },
      error: () => {
        // On error, still mark as signed out locally
        this.providerAuthStatus[provider] = false;
      }
    });
  }

  onConnectToFolder(): void {
    if (!this.folderLink.trim()) {
      this.errorMessage = 'Please enter a folder link';
      return;
    }

    // Validate and detect provider from URL
    const validationResult = this.validateAndDetectProvider(this.folderLink.trim());
    
    if (!validationResult.isValid) {
      this.errorMessage = validationResult.error || 'Please enter a valid OneDrive or Google Drive folder link';
      return;
    }

    this.selectedProvider = validationResult.provider!;
    this.errorMessage = '';
    this.isLoading = true;

    // Store folder link and provider for later use
    sessionStorage.setItem('folderLink', this.folderLink);
    sessionStorage.setItem('provider', this.selectedProvider);

    // Clear any existing reference image before proceeding
    const sessionId = this.authService.getOrCreateSessionId();
    this.faceService.clearReferenceImage(sessionId).subscribe({
      next: () => {
        // Reference image cleared, now check session validity
        this.checkSessionAndProceed();
      },
      error: () => {
        // Even if clearing fails, proceed
        // (reference image might not exist, which is fine)
        this.checkSessionAndProceed();
      }
    });
  }

  private checkSessionAndProceed(): void {
    // Check if session is already valid for this provider
    this.authService.validateSession(this.selectedProvider).subscribe({
      next: (validation) => {
        if (validation.valid && !validation.requires_auth) {
          // Session is valid, show loading screen and skip OAuth
          this.showProcessingLoader = true;
          this.fetchFolderContents();
        } else {
          // Need to authenticate
          this.initiateOAuth();
        }
      },
      error: () => {
        // On error, assume we need to authenticate
        this.initiateOAuth();
      }
    });
  }

  private initiateOAuth(): void {
    const authUrl = this.authService.initiateOAuth(this.selectedProvider);
    window.location.href = authUrl;
  }

  private fetchFolderContents(): void {
    const sessionId = this.authService.getSessionId();
    if (!sessionId) {
      this.showProcessingLoader = false;
      this.initiateOAuth();
      return;
    }

    this.storageService.getFolderContents(this.folderLink, sessionId, this.selectedProvider).subscribe({
      next: (response) => {
        this.isLoading = false;
        this.showProcessingLoader = false;
        
        // Navigate directly to search page with folder data
        this.router.navigate(['/search'], {
          state: { 
            contents: response.contents,
            folder: response.folder,
            provider: this.selectedProvider,
            folderLink: this.folderLink
          }
        });
      },
      error: (error) => {
        // If folder fetch fails, try OAuth
        this.showProcessingLoader = false;
        this.initiateOAuth();
      }
    });
  }

  private validateAndDetectProvider(link: string): { isValid: boolean; provider?: 'onedrive' | 'googledrive'; error?: string } {
    try {
      const url = new URL(link);
      const host = url.hostname.toLowerCase();
      const path = url.pathname;

      // OneDrive validation
      const oneDriveHosts = ['1drv.ms', 'onedrive.live.com', 'onedrive.com', 'd.docs.live.net'];
      if (oneDriveHosts.some(h => host === h || host.endsWith('.' + h))) {
        // For 1drv.ms short links, validate folder path structure
        if (host === '1drv.ms') {
          if (!path.startsWith('/f/')) {
            return { isValid: false, error: 'OneDrive link must be a folder link (should start with /f/)' };
          }
          // Ensure there's content after /f/
          if (path.length <= 3) {
            return { isValid: false, error: 'OneDrive link appears incomplete' };
          }
        }
        return { isValid: true, provider: 'onedrive' };
      }

      // Google Drive validation
      const googleDriveHosts = ['drive.google.com', 'docs.google.com'];
      if (googleDriveHosts.some(h => host === h || host.endsWith('.' + h))) {
        // Validate that it's actually a folder link
        const hasFolderPath = path.includes('/folders/');
        const hasDrivePath = path.includes('/drive/');
        const hasIdParam = url.searchParams.has('id');
        
        if (!hasFolderPath && !hasDrivePath && !hasIdParam) {
          return { isValid: false, error: 'Google Drive link must be a folder link (should contain /folders/ or /drive/)' };
        }
        return { isValid: true, provider: 'googledrive' };
      }

      return { isValid: false, error: 'URL must be from OneDrive (1drv.ms, onedrive.live.com) or Google Drive (drive.google.com)' };
    } catch (e) {
      return { isValid: false, error: 'Invalid URL format. Please enter a complete URL starting with https://' };
    }
  }
}