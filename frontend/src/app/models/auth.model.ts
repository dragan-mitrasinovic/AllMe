export interface Token {
  access_token: string;
  provider: 'onedrive' | 'googledrive';
  scope?: string;
}

export interface UserSession {
  session_id: string;
  tokens: { [provider: string]: Token };
}

export interface CloudItem {
  id: string;
  name: string;
  mime_type: string;
  is_folder: boolean;
  provider: 'onedrive' | 'googledrive';
  download_url: string;
  face_recognition_optimized_url?: string;      // 800px optimized for face recognition
  thumbnail_url?: string;    // 400px for frontend display
  match_distance?: number;   // Face recognition match distance (0.0-1.0, lower is better)
}