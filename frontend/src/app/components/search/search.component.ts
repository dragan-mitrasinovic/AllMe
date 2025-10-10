import { Component, OnInit, OnDestroy, inject, ViewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatGridListModule } from '@angular/material/grid-list';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatRadioModule } from '@angular/material/radio';
import { MatSnackBarModule } from '@angular/material/snack-bar';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatPaginatorModule } from '@angular/material/paginator';
import { FormsModule } from '@angular/forms';
import { HttpClient } from '@angular/common/http';
import { BreakpointObserver, Breakpoints } from '@angular/cdk/layout';
import { CloudItem } from '../../models/auth.model';
import { FaceService } from '../../services/face.service';
import { AuthService } from '../../services/auth.service';
import { NotificationService } from '../../services/notification.service';
import { ThumbnailPipe } from '../../pipes/thumbnail.pipe';

@Component({
  selector: 'app-search',
  templateUrl: './search.component.html',
  styleUrl: './search.component.css',
  imports: [
    CommonModule,
    MatCardModule,
    MatGridListModule,
    MatButtonModule,
    MatIconModule,
    MatProgressBarModule,
    MatCheckboxModule,
    MatTooltipModule,
    MatRadioModule,
    MatSnackBarModule,
    MatProgressSpinnerModule,
    MatPaginatorModule,
    FormsModule,
    ThumbnailPipe
  ]
})
export class SearchComponent implements OnInit, OnDestroy {
  @ViewChild('uploadSection', { read: ElementRef }) uploadSection!: ElementRef;

  private readonly router = inject(Router);
  private readonly faceService = inject(FaceService);
  private readonly authService = inject(AuthService);
  private readonly breakpointObserver = inject(BreakpointObserver);
  private readonly notificationService = inject(NotificationService);
  private readonly http = inject(HttpClient);

  provider: 'onedrive' | 'googledrive' = 'onedrive';
  contents: CloudItem[] = [];
  images: CloudItem[] = [];
  folder: CloudItem | null = null;
  folderLink: string = '';
  selectedFile: File | null = null;
  selectedCloudImage: CloudItem | null = null;
  referenceImageUrl: string | null = null;  // URL of the selected reference image for display
  referenceImageProvider: 'onedrive' | 'googledrive' | null = null;  // Provider for the reference image
  errorMessage: string = '';
  isUploading: boolean = false;
  uploadComplete: boolean = false;
  referenceImageSource: 'device' | 'folder' = 'device';
  isSelectingFromFolder: boolean = false;
  isMatching: boolean = false;
  matchingProgress: number = 0;
  currentImage: number = 0;
  totalImages: number = 0;
  matchesFound: number = 0;
  matchingMessage: string = '';
  gridCols: number = 4;
  recursiveSearch: boolean = false;

  // Pagination
  currentPage: number = 1;
  itemsPerPage: number = 24;
  totalPages: number = 1;
  paginatedContents: CloudItem[] = [];

  ngOnInit(): void {
    const navigation = this.router.getCurrentNavigation();
    const state = navigation?.extras?.state || history.state;

    // Try to get from navigation state first, then from sessionStorage
    if (state && state.contents) {
      this.contents = state.contents;
      this.images = state.contents.filter((item: CloudItem) => !item.is_folder && this.isImage(item.mime_type));
      this.folder = state.folder;
      this.provider = state.provider;
      this.folderLink = state.folderLink || '';
      
      // Save to sessionStorage for navigation back from results
      sessionStorage.setItem('searchContext', JSON.stringify({
        contents: this.contents,
        folder: this.folder,
        provider: this.provider,
        folderLink: this.folderLink
      }));
      
      // Initialize pagination
      this.updatePagination();
    } else {
      // Try to restore from sessionStorage
      const savedContext = sessionStorage.getItem('searchContext');
      if (savedContext) {
        const context = JSON.parse(savedContext);
        this.contents = context.contents;
        this.images = context.contents.filter((item: CloudItem) => !item.is_folder && this.isImage(item.mime_type));
        this.folder = context.folder;
        this.provider = context.provider;
        this.folderLink = context.folderLink || '';
        this.updatePagination();
      } else {
        this.errorMessage = 'No folder data available. Please start over.';
      }
    }

    this.breakpointObserver.observe([
      Breakpoints.XSmall,
      Breakpoints.Small,
      Breakpoints.Medium,
      Breakpoints.Large,
      Breakpoints.XLarge
    ]).subscribe(result => {
      if (result.breakpoints[Breakpoints.XSmall]) {
        this.gridCols = 2;
      } else if (result.breakpoints[Breakpoints.Small]) {
        this.gridCols = 3;
      } else if (result.breakpoints[Breakpoints.Medium]) {
        this.gridCols = 4;
      } else {
        this.gridCols = 6;
      }
    });
  }

  updatePagination(): void {
    this.totalPages = Math.ceil(this.contents.length / this.itemsPerPage);
    const startIndex = (this.currentPage - 1) * this.itemsPerPage;
    const endIndex = startIndex + this.itemsPerPage;
    this.paginatedContents = this.contents.slice(startIndex, endIndex);
  }

  goToPage(page: number): void {
    if (page < 1 || page > this.totalPages) return;
    this.currentPage = page;
    this.updatePagination();
    // Scroll to top of gallery
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  getPageNumbers(): number[] {
    const pages: number[] = [];
    const maxPagesToShow = 7;
    
    if (this.totalPages <= maxPagesToShow) {
      // Show all pages if total is small
      for (let i = 1; i <= this.totalPages; i++) {
        pages.push(i);
      }
    } else {
      // Show first page
      pages.push(1);
      
      // Calculate range around current page
      let startPage = Math.max(2, this.currentPage - 2);
      let endPage = Math.min(this.totalPages - 1, this.currentPage + 2);
      
      // Add ellipsis after first page if needed
      if (startPage > 2) {
        pages.push(-1); // -1 represents ellipsis
      }
      
      // Add pages around current
      for (let i = startPage; i <= endPage; i++) {
        pages.push(i);
      }
      
      // Add ellipsis before last page if needed
      if (endPage < this.totalPages - 1) {
        pages.push(-1); // -1 represents ellipsis
      }
      
      // Show last page
      pages.push(this.totalPages);
    }
    
    return pages;
  }

  isImage(mimeType: string): boolean {
    return mimeType.startsWith('image/');
  }

  isFolder(item: CloudItem): boolean {
    return item.is_folder;
  }

  getItemIcon(item: CloudItem): string {
    if (item.is_folder) return 'folder';
    if (this.isImage(item.mime_type)) return 'image';
    return 'insert_drive_file';
  }

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files.length > 0) {
      this.selectedFile = input.files[0];

      // Create preview URL for display
      if (this.referenceImageUrl) {
        URL.revokeObjectURL(this.referenceImageUrl);
      }
      this.referenceImageUrl = URL.createObjectURL(this.selectedFile);
      this.referenceImageProvider = null;  // Blob URLs don't need provider

      this.uploadReferenceImage();
    }
  }

  uploadReferenceImage(): void {
    if (!this.selectedFile) return;

    const sessionId = this.authService.getSessionId();
    if (!sessionId) {
      this.errorMessage = 'Session expired. Please start over.';
      return;
    }

    const previousReferenceUrl = this.referenceImageUrl;

    this.isUploading = true;
    this.errorMessage = '';
    this.uploadComplete = false;

    this.faceService.registerBaseFace(sessionId, this.selectedFile).subscribe({
      next: (response) => {
        this.isUploading = false;
        this.uploadComplete = response.success;
        this.notificationService.showSuccess('Reference image uploaded successfully');

        // Scroll to upload section after successful upload
        setTimeout(() => {
          this.scrollToUploadSection();
        }, 100);
      },
      error: (error) => {
        this.isUploading = false;
        this.uploadComplete = false;

        // Restore previous preview on error
        if (this.referenceImageUrl && this.referenceImageUrl !== previousReferenceUrl) {
          URL.revokeObjectURL(this.referenceImageUrl);
        }
        this.referenceImageUrl = previousReferenceUrl;
        this.selectedFile = null;

        this.errorMessage = error.message || 'Failed to upload reference image. Please try again.';
        this.notificationService.showError(this.errorMessage);

        // Scroll to upload section to show error message
        setTimeout(() => {
          this.scrollToUploadSection();
        }, 100);
      }
    });
  }

  startFaceMatching(): void {
    if (!this.uploadComplete) return;

    const sessionId = this.authService.getSessionId();

    if (!sessionId || !this.folderLink) {
      this.errorMessage = 'Session expired. Please start over.';
      return;
    }

    this.isMatching = true;
    this.errorMessage = '';
    this.matchingProgress = 0;
    this.matchingMessage = 'Starting face matching...';

    this.faceService.compareFolder(sessionId, this.folderLink, this.provider, this.recursiveSearch).subscribe({
      next: (response) => {
        const jobId = response.job_id;

        this.faceService.pollJobStatus(jobId).subscribe({
          next: (status) => {
            this.matchingProgress = status.progress;
            this.currentImage = status.current_image;
            this.totalImages = status.total_images;
            this.matchesFound = status.matches_found;
            this.matchingMessage = status.message;

            if (status.status === 'completed' && status.matches) {
              this.isMatching = false;
              this.router.navigate(['/results'], {
                state: {
                  matches: status.matches,
                  provider: this.provider,
                  totalImages: status.total_images,
                  totalMatches: status.matches_found
                }
              });
            } else if (status.status === 'failed') {
              this.isMatching = false;
              this.errorMessage = status.error || 'Face matching failed. Please try again.';
            }
          },
          error: (error) => {
            this.isMatching = false;
            this.errorMessage = error.message || 'Failed to get job status. Please try again.';
          }
        });
      },
      error: (error) => {
        this.isMatching = false;
        this.errorMessage = error.message || 'Failed to start face matching. Please try again.';
      }
    });
  }

  onReferenceSourceChange(): void {
    // Clear error and upload status when switching source
    this.errorMessage = '';
    this.uploadComplete = false;

    if (this.referenceImageSource === 'folder') {
      this.isSelectingFromFolder = true;
      // Clear device upload reference but keep preview
      this.selectedFile = null;
    } else {
      this.isSelectingFromFolder = false;
      // Clear folder selection reference but keep preview
      this.selectedCloudImage = null;
    }
  }

  enableFolderSelection(): void {
    this.isSelectingFromFolder = true;
  }

  onImageClick(item: CloudItem): void {
    if (!this.isSelectingFromFolder || item.is_folder) {
      return;
    }

    // Keep selection mode active - will be disabled only on success
    // Download and upload the selected image
    this.uploadCloudImage(item);
  }

  uploadCloudImage(item: CloudItem): void {
    const sessionId = this.authService.getSessionId();
    if (!sessionId) {
      this.errorMessage = 'Session expired. Please start over.';
      return;
    }

    const previousCloudImage = this.selectedCloudImage;
    const previousReferenceUrl = this.referenceImageUrl;
    const previousProvider = this.referenceImageProvider;

    // Show processing notification
    this.notificationService.showInfo(`Processing ${item.name}...`);

    this.isUploading = true;
    this.errorMessage = '';
    this.uploadComplete = false;

    // Download the image from cloud storage using the download URL
    this.http.get(item.download_url, { responseType: 'blob' }).subscribe({
      next: (blob) => {
        // Convert blob to file
        const file = new File([blob], item.name, { type: item.mime_type });

        // Upload to face service
        this.faceService.registerBaseFace(sessionId, file).subscribe({
          next: (response) => {
            this.isUploading = false;
            this.uploadComplete = response.success;

            // Only update preview and selection after successful validation
            this.selectedCloudImage = item;
            // Use thumbnail for display (400px optimized)
            this.referenceImageUrl = item.thumbnail_url || item.download_url;
            this.referenceImageProvider = item.provider;

            // Exit selection mode on success
            this.isSelectingFromFolder = false;

            this.notificationService.showSuccess(`✓ Reference image ready: ${item.name}`);

            // Scroll to upload section after everything is loaded and ready
            setTimeout(() => {
              this.scrollToUploadSection();
            }, 100);
          },
          error: (error) => {
            this.isUploading = false;
            this.uploadComplete = false;

            // Keep previous selection on error - don't change anything
            this.selectedCloudImage = previousCloudImage;
            this.referenceImageUrl = previousReferenceUrl;
            this.referenceImageProvider = previousProvider;

            // Stay in selection mode so user can try another image
            this.isSelectingFromFolder = true;

            this.errorMessage = error.message || 'Failed to upload reference image. Please try again.';
            this.notificationService.showError(this.errorMessage);

            // Scroll to upload section to show error message
            setTimeout(() => {
              this.scrollToUploadSection();
            }, 100);
          }
        });
      },
      error: () => {
        this.isUploading = false;
        this.uploadComplete = false;

        // Keep previous selection on error
        this.selectedCloudImage = previousCloudImage;
        this.referenceImageUrl = previousReferenceUrl;
        this.referenceImageProvider = previousProvider;

        // Stay in selection mode so user can try another image
        this.isSelectingFromFolder = true;

        this.errorMessage = 'Failed to download image from cloud storage';
        this.notificationService.showError(this.errorMessage);

        // Scroll to upload section to show error message
        setTimeout(() => {
          this.scrollToUploadSection();
        }, 100);
      }
    });
  }

  private scrollToUploadSection(): void {
    if (this.uploadSection) {
      this.uploadSection.nativeElement.scrollIntoView({
        behavior: 'smooth',
        block: 'center'
      });
    }
  }

  isImageSelected(item: CloudItem): boolean {
    return this.selectedCloudImage?.id === item.id;
  }

  ngOnDestroy(): void {
    // Clean up blob URL if it was created from uploaded file (not cloud image)
    if (this.referenceImageUrl && !this.selectedCloudImage && this.referenceImageUrl.startsWith('blob:')) {
      URL.revokeObjectURL(this.referenceImageUrl);
    }
  }
}
