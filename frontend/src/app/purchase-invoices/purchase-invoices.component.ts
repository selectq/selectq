import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { AgGridAngular } from 'ag-grid-angular';
import { ColDef, CellClickedEvent, ICellRendererParams } from 'ag-grid-community';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { Subscription, forkJoin, of } from 'rxjs';
import { ApiService, PurchaseInvoice } from '../api.service';

@Component({ selector: 'app-purchase-invoices', standalone: true, imports: [CommonModule, FormsModule, RouterLink, AgGridAngular],
 templateUrl: './purchase-invoices.component.html', styleUrl: './purchase-invoices.component.css' })
export class PurchaseInvoicesComponent implements OnInit, OnDestroy {
 invoices: PurchaseInvoice[] = [];
 isDarkMode = true;
 private themeObserver?: MutationObserver;
 defaultColDef: ColDef<PurchaseInvoice> = { sortable: true, filter: true, resizable: true, wrapHeaderText: true, autoHeaderHeight: true };
 columnDefs: ColDef<PurchaseInvoice>[] = [
  { field: 'invoice_date', headerName: 'Invoice Date', width: 140, initialSort: 'desc' },
  { field: 'invoice_number', headerName: 'Invoice Number', width: 165 },
  { field: 'party_name', headerName: 'Party Name', minWidth: 190, flex: 1 },
  { field: 'party_address', headerName: 'Party Address', minWidth: 230, flex: 2, wrapText: true, autoHeight: true, cellStyle: { whiteSpace: 'pre-wrap', lineHeight: '1.5', paddingTop: '8px', paddingBottom: '8px' } },
  { field: 'party_gstin', headerName: 'Party GSTIN', width: 185 },
  { field: 'total_amount', headerName: 'Total Invoice Amount (INR)', width: 185, filter: 'agNumberColumnFilter', cellStyle: { textAlign: 'right' },
    valueFormatter: p => p.value == null ? '' : Number(p.value).toLocaleString('en-IN', {minimumFractionDigits: 2, maximumFractionDigits: 2}) },
  { colId: 'documents', headerName: 'Invoice Document', width: 230, autoHeight: true, sortable: false, filter: false,
    cellRenderer: (p: ICellRendererParams<PurchaseInvoice>) => {
     const container = document.createElement('div');
     const invoice = p.data;
     if (!invoice) return container;
     const link = (label: string, href: string, external = false) => {
      const a = document.createElement('a'); a.textContent = label; a.href = href;
      a.style.display = 'block'; a.style.color = 'var(--primary-color)';
      if (external) { a.target = '_blank'; a.rel = 'noopener noreferrer'; }
      container.appendChild(a);
     };
     if (invoice.file_name) link(invoice.file_name, `/api/purchase-invoices/${invoice.id}/file`);
     if (invoice.external_url && /^https?:\/\//i.test(invoice.external_url)) link('Open external invoice', invoice.external_url, true);
     if (!container.childElementCount) container.textContent = 'Not provided';
     return container;
    } },
  { colId: 'edit', headerName: '', width: 90, sortable: false, filter: false,
    cellRenderer: () => { const button = document.createElement('button'); button.type = 'button'; button.textContent = 'Edit'; button.className = 'grid-edit-btn'; return button; } }
 ];
 onGridCellClicked(event: CellClickedEvent<PurchaseInvoice>) {
  if (event.column.getColId() === 'edit' && event.data && !this.saving) {
   this.edit(event.data);
   setTimeout(() => document.getElementById('purchase-invoice-editor')?.scrollIntoView({behavior: 'smooth', block: 'start'}));
  }
 }
 partyLookupMessage = '';
 private partyLookup?: Subscription;
 gstinChanged(value: string) {
  this.partyLookup?.unsubscribe(); this.partyLookupMessage = '';
  const draft = this.draft;
  if (!draft || this.saving) return;
  const gstin = value.trim().toUpperCase(); draft.party_gstin = gstin;
  if (!/^[0-9]{2}[A-Z0-9]{13}$/.test(gstin)) return;
  const name = draft.party_name, address = draft.party_address;
  this.partyLookupMessage = 'Looking up saved party details...';
  this.partyLookup = this.api.getPurchaseParty(gstin).subscribe({
   next: party => {
    if (this.draft !== draft || draft.party_gstin !== gstin || this.saving) return;
    if (draft.party_name !== name || draft.party_address !== address) {
     this.partyLookupMessage = 'Saved party found. Your edits have been kept.'; return;
    }
    draft.party_name = party.party_name; draft.party_address = party.party_address;
    this.partyLookupMessage = 'Party details filled from saved invoices. You can edit them before saving.';
   },
   error: err => {
    if (this.draft !== draft || draft.party_gstin !== gstin) return;
    this.partyLookupMessage = err.status === 404 ? 'No saved party for this GSTIN. Enter the details; they will be remembered when you save.' : 'Party lookup unavailable. You can enter the details manually.';
   }
  });
 }
 sourceTransactionId: number | null = null;
 selectedInvoiceId: number | null = null;
 get unlinkedInvoices() { return this.invoices.filter(i => i.bank_transaction_id == null); }
 linkExisting() {
  if (!this.sourceTransactionId || !this.selectedInvoiceId || this.saving) return;
  const id = this.selectedInvoiceId;
  const transactionId = this.sourceTransactionId;
  this.saving = true; this.error = ''; this.message = '';
  this.requests.add(this.api.linkPurchaseInvoice(id, transactionId).subscribe({
   next: () => {
    this.invoices = this.invoices.map(i => i.id === id ? { ...i, bank_transaction_id: transactionId } : i);
    this.draft = null; this.file = null; this.sourceTransactionId = null; this.selectedInvoiceId = null;
    this.saving = false; this.message = 'Existing purchase invoice linked to the withdrawal.';
   },
   error: err => { this.saving = false; this.error = typeof err.error === 'string' ? err.error : 'Could not link purchase invoice.'; }
  }));
 }
 draft: PurchaseInvoice | null = null;
 file: File | null = null;
 loading = false; saving = false; error = ''; message = ''; source = ''; importId: number | null = null;
 private requests = new Subscription();
 constructor(private api: ApiService, private route: ActivatedRoute) {}
 ngOnInit() {
  const updateTheme = () => this.isDarkMode = document.body.getAttribute('data-theme') !== 'light';
  updateTheme(); this.themeObserver = new MutationObserver(updateTheme);
  this.themeObserver.observe(document.body, {attributes: true, attributeFilter: ['data-theme']});
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
    this.sourceTransactionId = txn.id;
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
 ngOnDestroy() { this.themeObserver?.disconnect(); this.partyLookup?.unsubscribe(); this.requests.unsubscribe(); }
 add() {
  this.partyLookup?.unsubscribe(); this.partyLookupMessage = '';
  this.draft = { invoice_number: '', invoice_date: '', party_name: '', party_address: '', party_gstin: '', total_amount: 0, file_name: '', bank_transaction_id: this.sourceTransactionId };
  this.file = null; this.source = ''; this.error = ''; this.message = '';
 }
 edit(invoice: PurchaseInvoice) { this.partyLookup?.unsubscribe(); this.partyLookupMessage = ''; this.draft = { ...invoice }; this.file = null; this.source = ''; this.error = ''; this.message = ''; }
 chooseFile(event: Event) {
  this.file = (event.target as HTMLInputElement).files?.[0] || null;
  this.error = '';
  if (this.file && (this.file.size > 10 * 1024 * 1024 || this.file.size === 0)) { this.error = 'Choose a non-empty PDF, JPEG or PNG up to 10 MB.'; this.file = null; }
 }
 save() {
  if (!this.draft || this.saving) return;
  this.saving = true; this.error = ''; this.message = '';
  this.requests.add(this.api.savePurchaseInvoice(this.draft, this.file).subscribe({ next: saved => {
   this.saving = false; this.draft = null; this.file = null; this.message = 'Purchase invoice saved.'; this.sourceTransactionId = null; this.selectedInvoiceId = null;
   this.requests.add(this.api.getPurchaseInvoices().subscribe({ next: rows => this.invoices = rows,
    error: () => this.error = 'Invoice saved, but the register could not be refreshed. Reload the page.' }));
  }, error: err => { this.saving = false; this.error = typeof err.error === 'string' ? err.error : 'Could not save purchase invoice.'; } }));
 }
}
