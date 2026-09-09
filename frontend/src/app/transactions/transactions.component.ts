import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiService, BankTransaction } from '../api.service';
import { ActivatedRoute, Router } from '@angular/router';

@Component({
  selector: 'app-transactions',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './transactions.component.html',
  styleUrls: ['./transactions.component.css']
})
export class TransactionsComponent implements OnInit {
  transactions: BankTransaction[] = [];
  isLoading = true;
  importId: number | null = null;

  constructor(
    private api: ApiService,
    private route: ActivatedRoute,
    private router: Router
  ) {}

  ngOnInit() {
    this.route.paramMap.subscribe(params => {
      const id = params.get('id');
      if (id) {
        this.importId = +id;
        this.fetchTransactions(this.importId);
      }
    });
  }

  fetchTransactions(id: number) {
    this.api.getTransactions(id).subscribe({
      next: (data) => {
        this.transactions = data || [];
        this.isLoading = false;
      },
      error: (err) => {
        console.error('Error fetching transactions', err);
        this.isLoading = false;
      }
    });
  }

  goBack() {
    this.router.navigate(['/history']);
  }
}
