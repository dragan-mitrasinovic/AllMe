import { Token, CloudItem } from './auth.model';

export interface DownloadRequest {
  files: CloudItem[];
  token: Token;
  format: 'single' | 'zip';
}