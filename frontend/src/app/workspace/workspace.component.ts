import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';

@Component({
  selector: 'app-workspace', standalone: true,
  imports: [CommonModule, RouterOutlet, RouterLink, RouterLinkActive],
  templateUrl: './workspace.component.html', styleUrls: ['./workspace.component.css']
})
export class WorkspaceComponent {
  isDarkMode = true;
  isSidebarCollapsed = false;
  constructor() {
    const savedTheme = localStorage.getItem('theme');
    if (savedTheme) this.isDarkMode = savedTheme === 'dark';
    this.applyTheme();
    this.isSidebarCollapsed = localStorage.getItem('sidebarCollapsed') === 'true';
  }
  toggleSidebar() {
    this.isSidebarCollapsed = !this.isSidebarCollapsed;
    localStorage.setItem('sidebarCollapsed', String(this.isSidebarCollapsed));
  }
  toggleTheme() {
    this.isDarkMode = !this.isDarkMode;
    localStorage.setItem('theme', this.isDarkMode ? 'dark' : 'light');
    this.applyTheme();
  }
  private applyTheme() {
    if (this.isDarkMode) document.body.removeAttribute('data-theme');
    else document.body.setAttribute('data-theme', 'light');
  }
}
