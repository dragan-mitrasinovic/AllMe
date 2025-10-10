import { Pipe, PipeTransform } from '@angular/core';
import { ThumbnailService } from '../services/thumbnail.service';

@Pipe({
  name: 'thumbnail',
  standalone: true
})
export class ThumbnailPipe implements PipeTransform {
  constructor(private thumbnailService: ThumbnailService) {}

  transform(url: string, provider: string): string {
    return this.thumbnailService.getProxiedThumbnailUrl(url, provider);
  }
}