import { CloudItem } from './auth.model';

export interface GetFolderContentsResponse {
  folder: CloudItem;
  contents: CloudItem[];
}

export interface FaceRegisterResponse {
  success: boolean;
}

export interface CompareFolderRequest {
  session_id: string;
  folder_link: string;
  provider: string;
  recursive?: boolean;
}

export interface CompareFolderResponse {
  job_id: string;
  status: string;
}

export interface JobStatusResponse {
  job_id: string;
  status: string;
  progress: number;
  current_image: number;
  total_images: number;
  matches_found: number;
  message: string;
  matches?: CloudItem[];
  error?: string;
}