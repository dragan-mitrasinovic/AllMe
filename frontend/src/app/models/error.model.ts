export interface ErrorResponse {
  code: string;
  message: string;
  details?: any;
  retryable: boolean;
  suggestedAction?: string;
}

export interface ApiError extends Error {
  code: string;
  retryable: boolean;
  suggestedAction?: string;
  details?: any;
}