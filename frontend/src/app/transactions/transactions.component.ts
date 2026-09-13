import { AfterViewChecked, Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, BankTransaction, BusinessPartner, SalesInvoice, InvoiceAllocation } from '../api.service';
import { ActivatedRoute, Router } from '@angular/router';
import { AgGridAngular } from 'ag-grid-angular';
import { ColDef, CellClickedEvent, CellValueChangedEvent, ValueSetterParams, ValueFormatterParams } from 'ag-grid-community';

@Component({
  selector: 'app-transactions',
  standalone: true,
  imports: [CommonModule, FormsModule, AgGridAngular],
  templateUrl: './transactions.component.html',
  styleUrls: ['./transactions.component.css']
})
export class TransactionsComponent implements OnInit, AfterViewChecked {
  @ViewChild('allocationPanel') allocationPanel?: ElementRef<HTMLElement>;
  private revealAllocation = false;

  onCellClicked(event: CellClickedEvent<BankTransaction>) {
    if (event.colDef.field === 'invoice_number' && event.data && !this.saving) {
      const txn = event.data;
      if (txn.purchase_invoice_id || this.canRegisterPurchase(txn)) {
        this.selected = null;
        this.router.navigate(['/purchase-invoices'], { queryParams: {
          transaction_id: txn.id, import_id: this.importId, id: txn.purchase_invoice_id || undefined
        } });
      } else if (this.canAllocateSales(txn)) this.openAllocation(txn);
    }
  }

  canAllocateSales(txn?: BankTransaction): boolean {
    return !!txn && !txn.purchase_invoice_id && txn.account_head?.trim().toLowerCase() === 'sales invoice' && txn.deposit_amt > 0 && !txn.withdrawal_amt;
  }
  canRegisterPurchase(txn?: BankTransaction): boolean {
    if (!txn || txn.withdrawal_amt <= 0 || txn.invoice_number?.trim()) return false;
    const head = (txn.account_head || '').trim().toLowerCase();
    return head !== 'sales invoice' && !/salary|personal/.test(`${head} ${txn.sub_account_head || ''}`.toLowerCase());
  }
  invoiceActionLabel(txn?: BankTransaction): string {
    if (txn?.purchase_invoice_id) return 'View purchase invoice';
    if (this.canRegisterPurchase(txn)) return 'Register purchase invoice...';
    if (this.canAllocateSales(txn)) return txn?.invoice_number || 'Allocate sales invoices...';
    return txn?.invoice_number || '';
  }

  ngAfterViewChecked() {
    if (this.revealAllocation && this.allocationPanel) {
      this.revealAllocation = false;
      this.allocationPanel.nativeElement.scrollIntoView({ block: 'start', behavior: 'smooth' });
      this.allocationPanel.nativeElement.focus({ preventScroll: true });
    }
  }
  transactions: BankTransaction[] = [];
  invoices: SalesInvoice[] = [];
  selected: BankTransaction | null = null;
  allocations: InvoiceAllocation[] = [];
  saving = false;
  allocationError = '';

  openAllocation(txn: BankTransaction) {
    if (this.saving || !this.canAllocateSales(txn)) return;
    this.selected = { ...txn };
    this.revealAllocation = true;
    this.allocations = (txn.allocations || []).map(a => ({ ...a }));
    this.allocationError = '';
    this.api.getSalesInvoices().subscribe({ next: data => this.invoices = data || [], error: err => this.allocationError = err.error || err.message });
  }
  get availableInvoices() {
    return this.invoices.filter(i => i.business_partner_id === this.selected?.business_partner_id &&
      ((!i.is_closed && !i.is_settled) || this.allocations.some(a => a.sales_invoice_id === i.id)));
  }
  allocationFor(id: number) { return this.allocations.find(a => a.sales_invoice_id === id); }
  setAllocation(id: number, value: number) {
    this.allocations = this.allocations.filter(a => a.sales_invoice_id !== id);
    if (value > 0) this.allocations.push({ sales_invoice_id: id, amount: value });
  }
  get receiptAmount() { return this.selected?.forex_amount || this.selected?.deposit_amt || 0; }
  get allocatedTotal() { return this.allocations.reduce((sum, a) => sum + a.amount, 0); }
  saveAllocations() {
    if (!this.selected) return;
    this.saving = true;
    this.api.updateTransaction(this.selected.id, { ...this.selected, account_head: 'Sales Invoice', allocations: this.allocations }).subscribe({
      next: () => { this.saving = false; this.selected = null; this.fetchTransactions(this.importId!); },
      error: err => { this.saving = false; this.allocationError = err.error || err.message; }
    });
  }

  businessPartners: BusinessPartner[] = [];
  isLoading = true;
  importId: number | null = null;
  isDarkMode = false;

  public columnDefs: ColDef[] = [
    { field: 'date', headerName: 'Date', width: 120, sortable: true, filter: true },
    { field: 'narration', headerName: 'Narration', flex: 2, sortable: true, filter: true, wrapText: true, autoHeight: true },
    { field: 'withdrawal_amt', headerName: 'Withdrawal', width: 130, sortable: true, filter: 'agNumberColumnFilter', valueFormatter: this.currencyFormatter },
    { field: 'deposit_amt', headerName: 'Deposit', width: 130, sortable: true, filter: 'agNumberColumnFilter', valueFormatter: this.currencyFormatter },
    { field: 'currency', headerName: 'Currency', width: 115, minWidth: 115, sortable: true, filter: true,
      valueGetter: params => params.data ? (params.data.currency?.trim().toUpperCase() || 'INR') : null },
    { colId: 'amount_in_currency', headerName: 'Amount in Currency', width: 180, sortable: true, filter: 'agNumberColumnFilter',
      headerTooltip: 'Original currency amount. INR rows use the deposit or withdrawal amount.',
      valueGetter: params => {
        const txn = params.data as BankTransaction | undefined;
        if (!txn) return null;
        const currency = txn.currency?.trim().toUpperCase() || 'INR';
        return currency === 'INR' ? (txn.deposit_amt || txn.withdrawal_amt || 0) : txn.forex_amount;
      },
      valueFormatter: params => params.value == null ? '' : Number(params.value).toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
      cellStyle: { textAlign: 'right' } },
    { field: 'invoice_number', headerName: 'Invoices (click to allocate)', editable: false, minWidth: 190, flex: 1, sortable: true, filter: true,
      cellStyle: params => ({ cursor: params.data?.purchase_invoice_id || this.canRegisterPurchase(params.data) || this.canAllocateSales(params.data) ? 'pointer' : 'default' }),
      valueFormatter: params => this.invoiceActionLabel(params.data),
      tooltipValueGetter: params => this.invoiceActionLabel(params.data) },
    { field: 'account_head', headerName: 'Account Head', editable: true, flex: 1, sortable: true, filter: true },
    { field: 'sub_account_head', headerName: 'Sub Account', editable: true, flex: 1, sortable: true, filter: true },
    { 
      field: 'business_partner_name', 
      headerName: 'Business Partner', 
      editable: (params) => params.data.account_head === 'Sales Invoice',
      flex: 1,
      cellEditor: 'agSelectCellEditor',
      cellEditorParams: () => {
        return {
          values: ['', ...this.businessPartners.map(bp => bp.name)]
        };
      }
    }
  ];

  public defaultColDef: ColDef = {
    resizable: true,
    minWidth: 130,
  };

  constructor(
    private api: ApiService,
    private route: ActivatedRoute,
    private router: Router
  ) {}

  ngOnInit() {
    // Determine theme on init
    this.isDarkMode = document.documentElement.getAttribute('data-theme') === 'dark';
    
    // Listen for theme changes from body/html
    const observer = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        if (mutation.attributeName === 'data-theme') {
          this.isDarkMode = document.documentElement.getAttribute('data-theme') === 'dark';
        }
      });
    });
    observer.observe(document.documentElement, { attributes: true });

    this.api.getBusinessPartners().subscribe({
      next: (data) => {
        this.businessPartners = data || [];
        // Force grid to re-render columns with new business partners if needed
        this.columnDefs = [...this.columnDefs];
      },
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

  currencyFormatter(params: ValueFormatterParams) {
    if (!params.value) return '';
    return Number(params.value).toFixed(2);
  }

  onCellValueChanged(event: CellValueChangedEvent) {
    const txn = event.data as BankTransaction;

    // Clear business partner if account head is changed from Sales Invoice
    if (event.column.getColId() === 'account_head' && txn.account_head !== 'Sales Invoice') {
      txn.business_partner_name = '';
      txn.business_partner_id = undefined;
      if (event.node) {
        event.api.refreshCells({ rowNodes: [event.node], columns: ['business_partner_name'] });
      }
    }

    // Find the business partner ID from the selected name
    const selectedBp = this.businessPartners.find(bp => bp.name === txn.business_partner_name);
    txn.business_partner_id = selectedBp ? selectedBp.id : undefined;

    const payload = {
      account_head: txn.account_head || '',
      sub_account_head: txn.sub_account_head || '',
      invoice_number: txn.invoice_number || '',
      business_partner_id: txn.business_partner_id || null
    };

    this.api.updateTransaction(txn.id, payload).subscribe({
      next: () => {
        console.log('Transaction updated successfully');
      },
      error: (err) => {
        alert('Failed to save changes: ' + err.message);
        this.fetchTransactions(this.importId!);
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
