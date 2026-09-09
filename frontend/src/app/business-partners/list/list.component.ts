import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiService, BusinessPartner } from '../../api.service';
import { Router } from '@angular/router';

@Component({
  selector: 'app-list',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './list.component.html',
  styleUrls: ['./list.component.css']
})
export class ListComponent implements OnInit {
  partners: BusinessPartner[] = [];
  selectedPartner: BusinessPartner | null = null;
  isLoading = true;

  constructor(private api: ApiService, private router: Router) {}

  ngOnInit() {
    this.api.getBusinessPartners().subscribe({
      next: (data) => {
        this.partners = data || [];
        this.isLoading = false;
        // Optionally auto-select the first one
        if (this.partners.length > 0) {
          this.selectedPartner = this.partners[0];
        }
      },
      error: (err) => {
        console.error('Error fetching partners', err);
        this.isLoading = false;
      }
    });
  }

  selectPartner(p: BusinessPartner) {
    this.selectedPartner = p;
  }

  addNew() {
    this.router.navigate(['/business-partners/new']);
  }

  editPartner(id: number) {
    this.router.navigate(['/business-partners/edit', id]);
  }
}
