import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { CanActivateChildFn, Router } from '@angular/router';
import { catchError, map, of, throwError } from 'rxjs';

export interface Session { configured: boolean; authenticated: boolean; name: string; email: string; }
@Injectable({ providedIn: 'root' })
export class AuthService {
  private http = inject(HttpClient);
  session() { return this.http.get<Session>('/auth/session'); }
}
export const workspaceGuard: CanActivateChildFn = () => {
  const router = inject(Router);
  return inject(AuthService).session().pipe(
    map(session => session.authenticated || router.createUrlTree(['/login'])),
    catchError(() => of(router.createUrlTree(['/login'], { queryParams: { error: 'unavailable' } })))
  );
};
export const sessionInterceptor: HttpInterceptorFn = (req, next) => {
  const router = inject(Router);
  return next(req).pipe(catchError((error: HttpErrorResponse) => {
    if (req.url.startsWith('/api/') && error.status === 401) {
      void router.navigate(['/login'], { queryParams: { error: 'expired' } });
    }
    return throwError(() => error);
  }));
};
