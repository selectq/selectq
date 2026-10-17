import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { AuthService, Session } from '../auth.service';

@Component({
  selector: 'app-login', standalone: true, imports: [CommonModule, RouterLink],
  template: `
    <main class="login-page">
      <a routerLink="/" class="wordmark">Select Q<span>DESIGN & INTERIORS</span></a>
      <section aria-labelledby="login-title">
        <p class="eyebrow">PRIVATE WORKSPACE</p>
        <h1 id="login-title">Welcome back.</h1>
        <p>Sign in with your approved Google account to access the Select Q workspace.</p>
        <p *ngIf="message" role="status" class="notice">{{ message }}</p>
        <p *ngIf="!session && !message" role="status">Checking sign-in availability…</p>
        <ng-container *ngIf="session">
          <a *ngIf="session.authenticated" routerLink="/upload" class="google-button">Open workspace <span>↗</span></a>
          <a *ngIf="!session.authenticated && session.configured" href="/auth/google" class="google-button"><span class="google-g" aria-hidden="true">G</span> Continue with Google <span>↗</span></a>
          <p *ngIf="!session.configured" class="notice">Google sign-in is being set up. Please contact the studio for access.</p>
        </ng-container>
        <p class="small">Access is limited to approved team members.</p>
        <a routerLink="/" class="back">← Back to the studio</a>
      </section>
      <footer>SELECT Q · TEAM ACCESS</footer>
    </main>`,
  styles: [`
    :host{display:block;color:#292b25}.login-page{min-height:100vh;background:#f3f1eb;padding:36px 6vw;display:flex;flex-direction:column}
    a{color:inherit;text-decoration:none}.wordmark{font:italic 34px Georgia,serif;width:fit-content}.wordmark span{display:block;font:9px Arial,sans-serif;letter-spacing:3px;margin-top:9px}
    section{width:min(100%,460px);margin:auto;padding:70px 0}h1{font:normal 54px Georgia,serif;margin:18px 0}p{line-height:1.75;color:#64665e}
    .eyebrow,footer{font-size:10px;letter-spacing:3px}.google-button{display:flex;justify-content:space-between;align-items:center;gap:14px;border:1px solid #999e91;background:white;padding:18px 22px;margin:30px 0;font-size:14px}
    .google-g{font:bold 22px Arial;color:#4285f4}.small{font-size:12px}.back{display:inline-block;margin-top:38px;font-size:13px}.notice{padding:15px;background:#e7e7dc;margin-top:20px}a:focus-visible{outline:3px solid #65724d;outline-offset:5px}footer{padding-top:20px}
  `]
})
export class LoginComponent {
  session?: Session;
  message = '';
  constructor() {
    const errors: Record<string, string> = {
      state: 'Your sign-in request expired. Please try again.', cancelled: 'Sign-in was cancelled. You can try again below.',
      denied: 'This Google account does not have workspace access. Please use your approved account.',
      unavailable: 'Sign-in is temporarily unavailable. Please try again shortly.', expired: 'Your session has expired. Please sign in again.'
    };
    this.message = errors[inject(ActivatedRoute).snapshot.queryParamMap.get('error') || ''] || '';
    inject(AuthService).session().subscribe({ next: session => this.session = session,
      error: () => this.message = errors['unavailable'] });
  }
}
