import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, BankTransaction, BusinessPartner } from '../api.service';
import { ActivatedRoute, Router } from '@angular/router';

@Component({
  selector: 'app-transactions',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './transactions.component.html',
  styleUrls: ['./transactions.component.css']
})
export class TransactionsComponent implements OnInit {
  transactions: BankTransaction[] = [];
  businessPartners: BusinessPartner[] = [];
  isLoading = true;
  importId: number | null = null;

  editingTxnId: number | null = null;
  editPayload = {
    account_head: '',
    sub_account_head: '',
    business_partner_id: null as number | null
  };

  constructor(
    private api: ApiService,
    private route: ActivatedRoute,
    private router: Router
  ) {}

  ngOnInit() {
    this.api.getBusinessPartners().subscribe({
      next: (data) => this.businessPartners = data || [],
      error: (err) => console.error(err)
    });

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

  startEdit(txn: BankTransaction) {
    this.editingTxnId = txn.id;
    this.editPayload = {
      account_head: txn.account_head || '',
      sub_account_head: txn.sub_account_head || '',
      business_partner_id: txn.business_partner_id || null
    };
  }

  cancelEdit() {
    this.editingTxnId = null;
  }

  saveEdit(txn: BankTransaction) {
    this.api.updateTransaction(txn.id, this.editPayload).subscribe({
      next: () => {
        txn.account_head = this.editPayload.account_head;
        txn.sub_account_head = this.editPayload.sub_account_head;
        txn.business_partner_id = this.editPayload.business_partner_id || undefined;
        if (txn.business_partner_id) {
          const bp = this.businessPartners.find(p => p.id === txn.business_partner_id);
          txn.business_partner_name = bp ? bp.name : '';
        } else {
          txn.business_partner_name = '';
        }
        this.editingTxnId = null;
      },
      error: (err) => alert('Failed to save: ' + err.message)
    });
  }

  goBack() {
    this.router.navigate(['/history']);
  }
}
