import { invoiceGSTTreatment, roundTax } from '../gst';
import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, SalesInvoice, BusinessPartner, InvoiceLineItem, CompanyProfile } from '../../api.service';
import { AgGridAngular } from 'ag-grid-angular';
import { ColDef, GridOptions, GridState, ValueFormatterParams } from 'ag-grid-community';

@Component({
  selector: 'app-sales-invoices-list',
  standalone: true,
  imports: [CommonModule, FormsModule, AgGridAngular],
  templateUrl: './list.component.html',
  styleUrl: './list.component.css'
})
export class ListComponent implements OnInit {
  invoices: SalesInvoice[] = [];
  businessPartners: BusinessPartner[] = [];
  currencies: string[] = ['INR', 'USD', 'GBP', 'EUR', 'AED', 'CHF'];
  isLoading = true;
  isSaving = false;
  isDarkMode = true;
  companyProfile: CompanyProfile | null = null;

  activeTab: 'view' | 'create' = 'view';
  isEditMode = false;

  gridState: GridState | undefined;
  readonly gridOptions: GridOptions<SalesInvoice> = {
    // Angular destroys the grid on tab changes and while refreshing invoices.
    // Register here so the state is captured before Angular tears it down.
    onGridPreDestroyed: (event) => { this.gridState = event.state; }
  };

  formInvoice: SalesInvoice = this.emptyInvoice();
  lineItems: InvoiceLineItem[] = [];
  paymentDueDays = '10';

  // Computed totals
  subtotal = 0;
  totalGst = 0;
  grandTotal = 0;

  get isINR(): boolean {
    return this.formInvoice.currency === 'INR';
  }

  get sellerGSTIN() { return this.isEditMode ? this.formInvoice.seller_gstin ?? '' : this.companyProfile?.gstin || ''; }
  get buyerGSTIN() { return this.selectedBPAddresses.find(a => a.id === this.formInvoice.address_id)?.gstin || ''; }
  get gstTreatment() { return invoiceGSTTreatment(this.formInvoice.currency, this.sellerGSTIN, this.buyerGSTIN); }
  get totalCGST() { return this.gstTreatment === 'intrastate' ? roundTax(roundTax(this.totalGst) / 2) : 0; }
  get totalSGST() { return this.gstTreatment === 'intrastate' ? roundTax(roundTax(this.totalGst) - this.totalCGST) : 0; }
  get totalIGST() { return this.gstTreatment === 'interstate' ? roundTax(this.totalGst) : 0; }
  taxPercent(item: InvoiceLineItem, tax: 'cgst' | 'sgst' | 'igst') {
    if (this.gstTreatment === 'none') return 0;
    return tax === 'igst' ? (this.gstTreatment === 'interstate' ? item.gst_percent : 0)
      : (this.gstTreatment === 'intrastate' ? item.gst_percent / 2 : 0);
  }

  get selectedBPContacts() {
    const bp = this.businessPartners.find(p => p.id === this.formInvoice.business_partner_id);
    return bp?.contacts || [];
  }

  get selectedBPAddresses() {
    return this.businessPartners.find(p => p.id === this.formInvoice.business_partner_id)?.addresses || [];
  }
  get activeAddresses() { return this.selectedBPAddresses.filter(a => !a.is_archived); }
  originalAddressID: number | null = null;
  originalPartnerID = 0;
  get retainedArchivedAddress() {
    return this.isEditMode && this.originalPartnerID === this.formInvoice.business_partner_id
      ? this.selectedBPAddresses.find(a => a.id === this.originalAddressID && a.is_archived) : undefined;
  }

  onBusinessPartnerChange() {
    const currency = this.businessPartners.find(p => p.id === this.formInvoice.business_partner_id)?.invoice_currency?.trim().toUpperCase();
    if (currency && currency !== this.formInvoice.currency) {
      if (!this.currencies.includes(currency)) this.currencies = [...this.currencies, currency];
      this.formInvoice.currency = currency;
      this.onCurrencyChange();
    }
    this.formInvoice.address_id = this.activeAddresses[0]?.id ?? null;
    const contacts = this.selectedBPContacts;
    if (contacts.length > 0) {
      const primary = contacts.find(c => c.is_primary) || contacts[0];
      this.formInvoice.contact_id = primary.id;
    } else {
      this.formInvoice.contact_id = undefined;
    }
  }

  onPaymentDueDaysChange(value: string) {
    this.paymentDueDays = value;
    const days = /^\d+$/.test(value) ? Number(value) : NaN;
    this.formInvoice.due_in_days = Number.isSafeInteger(days) ? days : undefined;
    this.onDateChange();
  }

  onDateChange() {
    this.formInvoice.financial_year = this.financialYear(this.formInvoice.invoice_date);
    const days = this.formInvoice.due_in_days;
    if (this.formInvoice.invoice_date && days !== undefined && Number.isInteger(days) && days >= 0) {
      const date = new Date(this.formInvoice.invoice_date);
      date.setUTCDate(date.getUTCDate() + days);
      this.formInvoice.due_date = Number.isFinite(date.getTime()) && date.getUTCFullYear() <= 9999
        ? date.toISOString().split('T')[0] : '';
    } else {
      this.formInvoice.due_date = '';
    }
  }

  // AG Grid Column Definitions
  public columnDefs: ColDef[] = [
    {
      field: 'invoice_number', headerName: 'Invoice #', flex: 1, minWidth: 170,
      pinned: 'left', lockPinned: true,
      sortable: true, filter: true, initialSort: 'asc',
      comparator: (a, b) => String(a ?? '').localeCompare(String(b ?? ''), 'en', { numeric: true })
    },
    { field: 'currency', headerName: 'Currency', width: 100, sortable: true, filter: true },
    {
      field: 'amount', headerName: 'Amount', width: 140, sortable: true,
      valueFormatter: (params: ValueFormatterParams) => {
        if (params.value == null) return '';
        return Number(params.value).toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
      },
      cellStyle: { textAlign: 'right' }
    },
    { field: 'invoice_date', headerName: 'Invoice Date', width: 130, sortable: true, filter: true },
    {
      field: 'business_partner_name', headerName: 'Business Partner & Address',
      flex: 2, minWidth: 280, sortable: true, filter: true,
      // Use the invoice's selected address version, including archived addresses.
      valueGetter: (params) => [params.data?.business_partner_name, params.data?.billing_address]
        .filter(Boolean).join('\n'),
      wrapText: true, autoHeight: true,
      cellStyle: { whiteSpace: 'pre-line', lineHeight: '20px', paddingTop: '8px', paddingBottom: '8px', overflowWrap: 'anywhere' }
    },
    { field: 'financial_year', headerName: 'FY', width: 110, sortable: true, filter: true },
    { field: 'due_date', headerName: 'Due Date', width: 120, sortable: true, filter: true },
    { field: 'is_closed', headerName: 'Closed', width: 100 },
    { field: 'expected_receipt', headerName: 'Expected receipt', width: 150 },
    { field: 'received_amount', headerName: 'Received', width: 130 },
    { field: 'outstanding_amount', headerName: 'Outstanding', width: 140 },
    {
      headerName: 'Items',
      width: 80,
      valueGetter: (params) => params.data?.line_items?.length || 0,
      cellStyle: { textAlign: 'center' }
    },
    {
      headerName: '',
      width: 80,
      cellRenderer: () => `<button class="grid-edit-btn" title="Edit">✏️</button>`,
      onCellClicked: (params) => {
        this.editInvoice(params.data);
      },
      sortable: false,
      filter: false,
      suppressSizeToFit: true
    },
    {
      headerName: '',
      width: 60,
      cellRenderer: () => `<button class="grid-edit-btn" title="Download PDF">📄</button>`,
      onCellClicked: (params) => {
        this.downloadPDF(params.data);
      },
      sortable: false,
      filter: false,
      suppressSizeToFit: true
    }
  ];

  public defaultColDef: ColDef = {
    resizable: true,
  };

  constructor(private api: ApiService) {}

  ngOnInit() {
    this.isDarkMode = !document.body.hasAttribute('data-theme') ||
                      document.body.getAttribute('data-theme') !== 'light';

    const observer = new MutationObserver(() => {
      this.isDarkMode = !document.body.hasAttribute('data-theme') ||
                        document.body.getAttribute('data-theme') !== 'light';
    });
    observer.observe(document.body, { attributes: true, attributeFilter: ['data-theme'] });

    this.loadData();
    this.api.getCompanyProfile().subscribe({
      next: (p) => this.companyProfile = p,
      error: (err) => console.error('Failed to load company profile', err)
    });
  }

  loadData() {
    this.isLoading = true;
    this.api.getBusinessPartners().subscribe({
      next: (bps) => {
        this.businessPartners = bps || [];
        this.api.getSalesInvoices().subscribe({
          next: (invs) => {
            this.invoices = invs || [];
            this.isLoading = false;
          },
          error: (err) => {
            console.error(err);
            this.isLoading = false;
          }
        });
      },
      error: (err) => {
        console.error(err);
        this.isLoading = false;
      }
    });
  }

  switchToCreate() {
    this.formInvoice = this.emptyInvoice();
    this.paymentDueDays = '10';
    this.lineItems = [];
    this.isEditMode = false;
    this.originalAddressID = null;
    this.originalPartnerID = 0;
    this.recalcTotals();
    this.onDateChange();
    this.activeTab = 'create';
  }

  editInvoice(inv: SalesInvoice) {
    this.formInvoice = { ...inv };
    this.paymentDueDays = String(inv.due_in_days ?? 0);
    this.formInvoice.due_in_days = inv.due_in_days ?? 0;
    this.originalAddressID = inv.address_id || null;
    this.originalPartnerID = inv.business_partner_id;
    this.lineItems = (inv.line_items || []).map(li => ({ ...li }));
    this.isEditMode = true;
    this.recalcTotals();
    this.onDateChange();
    this.activeTab = 'create';
  }

  cancelForm() {
    this.activeTab = 'view';
    this.isEditMode = false;
    this.formInvoice = this.emptyInvoice();
    this.paymentDueDays = '10';
    this.lineItems = [];
  }

  // Line item management
  addLineItem() {
    this.lineItems.push({
      description: this.isINR ? '' : 'Architectural Services for CAD Documentation',
      hsn_sac_code: '',
      quantity: 1,
      rate: 0,
      gst_percent: this.isINR ? 18 : 0,
      amount: 0
    });
  }

  removeLineItem(index: number) {
    this.lineItems.splice(index, 1);
    this.recalcTotals();
  }

  recalcLineAmount(item: InvoiceLineItem) {
    item.amount = item.quantity * item.rate;
    this.recalcTotals();
  }

  onCurrencyChange() {
    // Reset GST when switching away from INR
    if (!this.isINR) {
      for (const item of this.lineItems) {
        item.gst_percent = 0;
        item.hsn_sac_code = '';
      }
    }
    this.recalcTotals();
  }

  recalcTotals() {
    this.subtotal = this.lineItems.reduce((sum, li) => sum + li.amount, 0);
    this.totalGst = this.isINR
      ? this.lineItems.reduce((sum, li) => sum + (li.amount * li.gst_percent / 100), 0)
      : 0;
    this.grandTotal = this.subtotal + this.totalGst;
  }

  save() {
    const inv = this.formInvoice;
    if (!inv.financial_year || !inv.business_partner_id ||
        !inv.invoice_date || !inv.currency) {
      alert('Please fill out all header fields.');
      return;
    }

    if (inv.due_in_days === undefined || !inv.due_date) {
      alert('Enter payment due days as a nonnegative whole number that produces a valid due date.');
      return;
    }

    const retainsLegacyEmpty = this.isEditMode && !this.originalAddressID && this.originalPartnerID === inv.business_partner_id;
    if (!inv.address_id && this.selectedBPAddresses.length && !retainsLegacyEmpty) {
      alert('Select an active billing address. Add one on the business partner if all addresses are archived.');
      return;
    }
    if (this.lineItems.length === 0) {
      alert('Please add at least one line item.');
      return;
    }

    // Validate line items
    for (let i = 0; i < this.lineItems.length; i++) {
      if (!this.lineItems[i].description) {
        alert(`Line item ${i + 1}: Description is required.`);
        return;
      }
      if (this.lineItems[i].amount <= 0) {
        alert(`Line item ${i + 1}: Amount must be greater than zero.`);
        return;
      }
    }

    // Attach line items and computed total
    inv.line_items = this.lineItems;
    inv.amount = this.grandTotal;

    this.isSaving = true;

    if (this.isEditMode && inv.id) {
      this.api.updateSalesInvoice(inv.id, inv).subscribe({
        next: () => {
          this.isSaving = false;
          this.activeTab = 'view';
          this.loadData();
        },
        error: (err) => {
          alert('Failed to update: ' + (typeof err.error === 'string' ? err.error : err.error?.message || err.message));
          this.isSaving = false;
        }
      });
    } else {
      this.api.createSalesInvoice(inv).subscribe({
        next: () => {
          this.isSaving = false;
          this.activeTab = 'view';
          this.loadData();
        },
        error: (err) => {
          alert('Failed to create: ' + (typeof err.error === 'string' ? err.error : err.error?.message || err.message));
          this.isSaving = false;
        }
      });
    }
  }

  private financialYear(date: string): string {
    if (!date) return '';
    const [year, month] = date.split('-').map(Number);
    const start = month < 4 ? year - 1 : year;
    return `${start}-${start + 1}`;
  }

  private emptyInvoice(): SalesInvoice {
    return {
      project_name: '', our_reference: '', your_reference: '', order_number: '',
      additional_information: '',
      invoice_number: '',
      financial_year: this.financialYear(new Date().toISOString().split('T')[0]),
      business_partner_id: 0,
      invoice_date: new Date().toISOString().split('T')[0],
      due_in_days: 10,
      currency: 'INR',
      amount: 0,
      line_items: []
    };
  }

  downloadPDF(inv: SalesInvoice) {
    if (!inv.id) { alert('Save the invoice before downloading its PDF.'); return; }
    this.api.downloadSalesInvoicePDF(inv.id).subscribe({
      next: (pdf) => {
        const url = URL.createObjectURL(pdf);
        const link = document.createElement('a');
        link.href = url;
        link.download = `Invoice-${inv.invoice_number.replace(/[^a-zA-Z0-9_-]/g, '-')}.pdf`;
        document.body.appendChild(link);
        link.click();
        link.remove();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
      },
      error: () => alert('Could not generate the invoice PDF. Please try again.')
    });
  }
}
