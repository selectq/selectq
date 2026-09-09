import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiService, ImportRecord } from '../api.service';
import { Router } from '@angular/router';

@Component({
  selector: 'app-history',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './history.component.html',
  styleUrls: ['./history.component.css']
})
export class HistoryComponent implements OnInit {
  imports: ImportRecord[] = [];
  isLoading = true;

  constructor(private api: ApiService, private router: Router) {}

  ngOnInit() {
    this.api.getImports().subscribe({
      next: (data) => {
        this.imports = data || [];
        this.isLoading = false;
      },
      error: (err) => {
        console.error('Error fetching imports', err);
        this.isLoading = false;
      }
    });
  }

  viewTransactions(id: number) {
    this.router.navigate(['/history', id]);
  }
}
