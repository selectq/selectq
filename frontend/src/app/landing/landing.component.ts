import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { studio } from './landing.content';

@Component({
  selector: 'app-landing', standalone: true, imports: [CommonModule, RouterLink],
  templateUrl: './landing.component.html', styleUrls: ['./landing.component.css']
})
export class LandingComponent {
  studio = studio;
  menuOpen = false;
  filter = 'All';
  categories = ['All', 'Hospitality', 'Interiors'];
  get projects() { return this.studio.projects.filter(project => this.filter === 'All' || project.category === this.filter); }
}
