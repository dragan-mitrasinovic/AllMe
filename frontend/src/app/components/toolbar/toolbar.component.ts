import { Component, inject } from '@angular/core';
import { Router, NavigationEnd } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { ThemeService } from '../../services/theme.service';
import { CommonModule } from '@angular/common';
import { filter } from 'rxjs/operators';

@Component({
  selector: 'app-toolbar',
  templateUrl: './toolbar.component.html',
  styleUrl: './toolbar.component.css',
  imports: [CommonModule, MatToolbarModule, MatIconModule, MatButtonModule]
})
export class ToolbarComponent {
  themeService = inject(ThemeService);
  private router = inject(Router);
  
  showBackButton = false;
  backButtonLabel = '';
  private currentRoute = '';

  constructor() {
    // Listen to route changes to show/hide back button
    this.router.events
      .pipe(filter(event => event instanceof NavigationEnd))
      .subscribe((event: any) => {
        this.currentRoute = event.url;
        this.updateBackButton();
      });
    
    // Initial check
    this.currentRoute = this.router.url;
    this.updateBackButton();
  }

  private updateBackButton(): void {
    if (this.currentRoute.startsWith('/search')) {
      this.showBackButton = true;
      this.backButtonLabel = 'Back to Home';
    } else if (this.currentRoute.startsWith('/results')) {
      this.showBackButton = true;
      this.backButtonLabel = 'Back to Search';
    } else {
      this.showBackButton = false;
    }
  }

  onBackClick(): void {
    if (this.currentRoute.startsWith('/results')) {
      this.router.navigate(['/search']);
    } else if (this.currentRoute.startsWith('/search')) {
      this.router.navigate(['/']);
    }
  }

  toggleTheme(): void {
    this.themeService.toggleTheme();
  }
}
