import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ActivatedRouteSnapshot, provideRouter, Router, RouterStateSnapshot, UrlTree } from '@angular/router';
import { Observable } from 'rxjs';
import { workspaceGuard } from './auth.service';

describe('Workspace access', () => {
  let http: HttpTestingController;
  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()] });
    http = TestBed.inject(HttpTestingController);
  });
  afterEach(() => http.verify());
  for (const authenticated of [false, true]) {
    it(`handles authenticated=${authenticated}`, () => {
      const result = TestBed.runInInjectionContext(() => workspaceGuard({} as ActivatedRouteSnapshot, {} as RouterStateSnapshot)) as Observable<boolean | UrlTree>;
      result.subscribe(value => expect(value === true ? true : TestBed.inject(Router).serializeUrl(value as UrlTree)).toBe(authenticated ? true : '/login'));
      http.expectOne('/auth/session').flush({ configured: true, authenticated });
    });
  }
  it('fails closed when the session service is unavailable', () => {
    const result = TestBed.runInInjectionContext(() => workspaceGuard({} as ActivatedRouteSnapshot, {} as RouterStateSnapshot)) as Observable<UrlTree>;
    result.subscribe(value => expect(TestBed.inject(Router).serializeUrl(value)).toBe('/login?error=unavailable'));
    http.expectOne('/auth/session').flush('Offline', { status: 503, statusText: 'Unavailable' });
  });
});
