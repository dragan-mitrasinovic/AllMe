import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatGridListModule } from '@angular/material/grid-list';
import { BreakpointObserver, Breakpoints } from '@angular/cdk/layout';
import { CloudItem } from '../../models/auth.model';
import { DownloadService } from '../../services/download.service';
import { AuthService } from '../../services/auth.service';
import { FaceService } from '../../services/face.service';
import { NotificationService } from '../../services/notification.service';
import { ThumbnailPipe } from '../../pipes/thumbnail.pipe';

@Component({
  selector: 'app-results',
  templateUrl: './results.component.html',
  styleUrl: './results.component.css',
  imports: [
    CommonModule,
    MatCardModule,
    MatCheckboxModule,
    MatButtonModule,
    MatIconModule,
    MatGridListModule,
    ThumbnailPipe
  ]
})
export class ResultsComponent implements OnInit {
  private readonly router = inject(Router);
  private readonly downloadService = inject(DownloadService);
  private readonly authService = inject(AuthService);
  private readonly faceService = inject(FaceService);
  private readonly breakpointObserver = inject(BreakpointObserver);
  private readonly notificationService = inject(NotificationService);

  matches: CloudItem[] = [];
  strongMatches: CloudItem[] = [];
  weakMatches: CloudItem[] = [];
  provider: 'onedrive' | 'googledrive' = 'onedrive';
  selectedStrongItems: Set<string> = new Set();
  selectedWeakItems: Set<string> = new Set();
  errorMessage: string = '';
  isDownloading: boolean = false;
  totalImages: number = 0;
  totalMatches: number = 0;
  gridCols: number = 4;

  readonly STRONG_MATCH_THRESHOLD = 0.5;  // Distance < 0.5 = strong match
  readonly ITEMS_PER_PAGE = 12;
  
  // Pagination state
  strongCurrentPage = 1;
  weakCurrentPage = 1;
  strongTotalPages = 1;
  weakTotalPages = 1;

  ngOnInit(): void {
    const navigation = this.router.getCurrentNavigation();
    const state = navigation?.extras?.state || history.state;
    
    if (state && state.matches) {
      this.matches = state.matches;
      this.provider = state.provider;
      this.totalImages = state.totalImages || 0;
      this.totalMatches = state.totalMatches || this.matches.length;
      
      // Separate matches into strong and weak based on distance
      // Strong: distance < 0.5, Weak: distance >= 0.5
      // If match_distance is undefined, treat as weak match (fallback for safety)
      this.strongMatches = this.matches.filter(item => 
        item.match_distance !== undefined && item.match_distance < this.STRONG_MATCH_THRESHOLD
      );
      this.weakMatches = this.matches.filter(item => 
        item.match_distance === undefined || item.match_distance >= this.STRONG_MATCH_THRESHOLD
      );
      
      // Sort by distance (best matches first)
      this.strongMatches.sort((a, b) => (a.match_distance || 0) - (b.match_distance || 0));
      this.weakMatches.sort((a, b) => (a.match_distance || 1) - (b.match_distance || 1));
      
      // Calculate total pages
      this.strongTotalPages = Math.ceil(this.strongMatches.length / this.ITEMS_PER_PAGE) || 1;
      this.weakTotalPages = Math.ceil(this.weakMatches.length / this.ITEMS_PER_PAGE) || 1;
    } else {
      this.errorMessage = 'No results available. Please start over.';
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

  trackByItemId(index: number, item: CloudItem): string {
    return item.id;
  }

  isSelected(item: CloudItem): boolean {
    return this.selectedStrongItems.has(item.id) || this.selectedWeakItems.has(item.id);
  }

  isStrongMatch(item: CloudItem): boolean {
    return item.match_distance !== undefined && item.match_distance < this.STRONG_MATCH_THRESHOLD;
  }

  toggleSelection(item: CloudItem): void {
    const isStrong = this.isStrongMatch(item);
    const selectedSet = isStrong ? this.selectedStrongItems : this.selectedWeakItems;
    
    if (selectedSet.has(item.id)) {
      selectedSet.delete(item.id);
    } else {
      selectedSet.add(item.id);
    }
  }

  selectAllStrong(): void {
    if (this.selectedStrongItems.size === this.strongMatches.length) {
      this.selectedStrongItems.clear();
    } else {
      this.strongMatches.forEach(item => this.selectedStrongItems.add(item.id));
    }
  }

  selectAllWeak(): void {
    if (this.selectedWeakItems.size === this.weakMatches.length) {
      this.selectedWeakItems.clear();
    } else {
      this.weakMatches.forEach(item => this.selectedWeakItems.add(item.id));
    }
  }

  selectAll(): void {
    const totalSelected = this.selectedStrongItems.size + this.selectedWeakItems.size;
    const totalMatches = this.strongMatches.length + this.weakMatches.length;
    
    if (totalSelected === totalMatches) {
      this.selectedStrongItems.clear();
      this.selectedWeakItems.clear();
    } else {
      this.strongMatches.forEach(item => this.selectedStrongItems.add(item.id));
      this.weakMatches.forEach(item => this.selectedWeakItems.add(item.id));
    }
  }

  get totalSelectedCount(): number {
    return this.selectedStrongItems.size + this.selectedWeakItems.size;
  }

  // Pagination getters
  get paginatedStrongMatches(): CloudItem[] {
    const start = (this.strongCurrentPage - 1) * this.ITEMS_PER_PAGE;
    const end = start + this.ITEMS_PER_PAGE;
    return this.strongMatches.slice(start, end);
  }

  get paginatedWeakMatches(): CloudItem[] {
    const start = (this.weakCurrentPage - 1) * this.ITEMS_PER_PAGE;
    const end = start + this.ITEMS_PER_PAGE;
    return this.weakMatches.slice(start, end);
  }

  get strongPageNumbers(): number[] {
    return this.getPageNumbers(this.strongCurrentPage, this.strongTotalPages);
  }

  get weakPageNumbers(): number[] {
    return this.getPageNumbers(this.weakCurrentPage, this.weakTotalPages);
  }

  private getPageNumbers(currentPage: number, totalPages: number): number[] {
    const pages: number[] = [];
    const maxVisible = 5;
    
    if (totalPages <= maxVisible) {
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      if (currentPage <= 3) {
        for (let i = 1; i <= 4; i++) pages.push(i);
        pages.push(-1); // ellipsis
        pages.push(totalPages);
      } else if (currentPage >= totalPages - 2) {
        pages.push(1);
        pages.push(-1); // ellipsis
        for (let i = totalPages - 3; i <= totalPages; i++) pages.push(i);
      } else {
        pages.push(1);
        pages.push(-1); // ellipsis
        pages.push(currentPage - 1);
        pages.push(currentPage);
        pages.push(currentPage + 1);
        pages.push(-1); // ellipsis
        pages.push(totalPages);
      }
    }
    
    return pages;
  }

  goToStrongPage(page: number): void {
    if (page >= 1 && page <= this.strongTotalPages) {
      this.strongCurrentPage = page;
    }
  }

  goToWeakPage(page: number): void {
    if (page >= 1 && page <= this.weakTotalPages) {
      this.weakCurrentPage = page;
    }
  }

  downloadSelected(): void {
    const selectedMatches = [
      ...this.strongMatches.filter(item => this.selectedStrongItems.has(item.id)),
      ...this.weakMatches.filter(item => this.selectedWeakItems.has(item.id))
    ];
    this.downloadFiles(selectedMatches);
  }

  downloadAll(): void {
    this.downloadFiles([...this.strongMatches, ...this.weakMatches]);
  }

  private downloadFiles(files: CloudItem[]): void {
    if (files.length === 0) return;

    const sessionId = this.authService.getSessionId();
    if (!sessionId) {
      this.errorMessage = 'Session expired. Please start over.';
      return;
    }

    this.isDownloading = true;
    this.errorMessage = '';

    this.downloadService.downloadZip(files, sessionId, this.provider).subscribe({
      next: (blob) => {
        this.downloadService.triggerDownload(blob, `allme-photos-${Date.now()}.zip`);
        this.isDownloading = false;
        this.notificationService.showSuccess(`Successfully downloaded ${files.length} photo(s)`);
      },
      error: (error) => {
        this.isDownloading = false;
        this.errorMessage = error.message || 'Failed to download files. Please try again.';
        this.notificationService.showError(this.errorMessage);
      }
    });
  }

  startOver(): void {
    // Clear folder context but keep authentication tokens
    this.authService.clearFolderContext();
    
    // Clear reference image for the session
    const sessionId = this.authService.getSessionId();
    if (sessionId) {
      // Fire and forget - don't wait for response
      this.faceService.clearReferenceImage(sessionId).subscribe();
    }
    
    this.router.navigate(['/']);
  }
}
