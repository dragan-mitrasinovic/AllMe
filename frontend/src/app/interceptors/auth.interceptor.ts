import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { AuthService } from '../services/auth.service';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const authService = inject(AuthService);
  const sessionId = authService.getSessionId();

  // Add session ID to requests if available
  if (sessionId && !req.url.includes('session_id=')) {
    const modifiedReq = req.clone({
      setParams: {
        session_id: sessionId
      }
    });
    return next(modifiedReq);
  }

  return next(req);
};
