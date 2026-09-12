import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { Subscription, forkJoin, of } from 'rxjs';
import { ApiService, PurchaseInvoice } from '../api.service';

@Component({ selector: 'app-purchase-invoices', standalone: true, imports: [CommonModule, FormsModule, RouterLink],
 templateUrl: './purchase-invoices.component.html', styleUrl: './purchase-invoices.component.css' })
export class PurchaseInvoicesComponent implements OnInit, OnDestroy {
 invoices: PurchaseInvoice[] = [];
 draft: PurchaseInvoice | null = null;
 file: File | null = null;
 loading = false; saving = false; error = ''; message = ''; source = ''; importId: number | null = null;
 private requests = new Subscription();
 constructor(private api: ApiService, private route: ActivatedRoute) {}
 ngOnInit() {
  const params = this.route.snapshot.queryParamMap;
  const transactionId = Number(params.get('transaction_id'));
  this.importId = Number(params.get('import_id')) || null;
  const id = Number(params.get('id'));
  this.loading = true;
  this.requests.add(forkJoin({ invoices: this.api.getPurchaseInvoices(),
    transactions: transactionId && this.importId ? this.api.getTransactions(this.importId) : of([])
  }).subscribe({ next: ({ invoices, transactions }) => {
   this.invoices = invoices; this.loading = false;
   const existing = invoices.find(p => id ? p.id === id : transactionId > 0 && p.bank_transaction_id === transactionId);
   if (existing) { this.edit(existing); return; }
   if (transactionId) {
    const txn = (transactions || []).find(t => t.id === transactionId);
    if (!txn) { this.error = 'The source withdrawal could not be found.'; return; }
    const head = (txn.account_head || '').trim().toLowerCase();
    const classification = `${head} ${txn.sub_account_head || ''}`.toLowerCase();
    if (txn.withdrawal_amt <= 0 || txn.invoice_number?.trim() || head === 'sales invoice' || /salary|personal/.test(classification)) {
     this.error = 'This transaction is not eligible for purchase registration.'; return;
    }
    this.add();
    this.draft!.bank_transaction_id = txn.id;
    this.draft!.total_amount = txn.withdrawal_amt;
    this.draft!.party_name = txn.business_partner_name || '';
    this.source = txn.narration;
    const match = txn.date.match(/^(\d{2})\/(\d{2})\/(\d{2}|\d{4})$/);
    this.draft!.invoice_date = /^\d{4}-\d{2}-\d{2}$/.test(txn.date) ? txn.date : match ? `${match[3].length === 2 ? '20' : ''}${match[3]}-${match[2]}-${match[1]}` : '';
   }
  }, error: () => { this.loading = false; this.error = 'Could not load purchase invoices. Please try again.'; } }));
 }
 ngOnDestroy() { this.requests.unsubscribe(); }
 add() {
  this.draft = { invoice_number: '', invoice_date: '', party_name: '', party_address: '', party_gstin: '', total_amount: 0, file_name: '', bank_transaction_id: null };
  this.file = null; this.source = ''; this.error = ''; this.message = '';
 }
 edit(invoice: PurchaseInvoice) { this.draft = { ...invoice }; this.file = null; this.source = ''; this.error = ''; this.message = ''; }
 chooseFile(event: Event) {
  this.file = (event.target as HTMLInputElement).files?.[0] || null;
  this.error = '';
  if (this.file && (this.file.size > 10 * 1024 * 1024 || this.file.size === 0)) { this.error = 'Choose a non-empty PDF, JPEG or PNG up to 10 MB.'; this.file = null; }
 }
 save() {
  if (!this.draft || this.saving) return;
  this.saving = true; this.error = ''; this.message = '';
  this.requests.add(this.api.savePurchaseInvoice(this.draft, this.file).subscribe({ next: saved => {
   this.saving = false; this.draft = null; this.file = null; this.message = 'Purchase invoice saved.';
   this.requests.add(this.api.getPurchaseInvoices().subscribe({ next: rows => this.invoices = rows,
    error: () => this.error = 'Invoice saved, but the register could not be refreshed. Reload the page.' }));
  }, error: err => { this.saving = false; this.error = typeof err.error === 'string' ? err.error : 'Could not save purchase invoice.'; } }));
 }
}
