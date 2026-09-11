import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, SalesInvoice, BusinessPartner, InvoiceLineItem, CompanyProfile } from '../../api.service';
import { AgGridAngular } from 'ag-grid-angular';
import { ColDef, ValueFormatterParams } from 'ag-grid-community';

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

  formInvoice: SalesInvoice = this.emptyInvoice();
  lineItems: InvoiceLineItem[] = [];

  // Computed totals
  subtotal = 0;
  totalGst = 0;
  grandTotal = 0;

  get isINR(): boolean {
    return this.formInvoice.currency === 'INR';
  }

  get selectedBPContacts() {
    const bp = this.businessPartners.find(p => p.id === this.formInvoice.business_partner_id);
    return bp?.contacts || [];
  }

  onBusinessPartnerChange() {
    const contacts = this.selectedBPContacts;
    if (contacts.length > 0) {
      const primary = contacts.find(c => c.is_primary) || contacts[0];
      this.formInvoice.contact_id = primary.id;
    } else {
      this.formInvoice.contact_id = undefined;
    }
  }

  onDateChange() {
    this.formInvoice.financial_year = this.financialYear(this.formInvoice.invoice_date);
    if (this.formInvoice.invoice_date && this.formInvoice.due_in_days !== undefined) {
      const date = new Date(this.formInvoice.invoice_date);
      date.setDate(date.getDate() + this.formInvoice.due_in_days);
      this.formInvoice.due_date = date.toISOString().split('T')[0];
    } else {
      this.formInvoice.due_date = this.formInvoice.invoice_date;
    }
  }

  // AG Grid Column Definitions
  public columnDefs: ColDef[] = [
    { field: 'invoice_number', headerName: 'Invoice #', flex: 1, sortable: true, filter: true },
    { field: 'financial_year', headerName: 'FY', width: 110, sortable: true, filter: true },
    { field: 'business_partner_name', headerName: 'Business Partner', flex: 1.5, sortable: true, filter: true },
    { field: 'invoice_date', headerName: 'Date', width: 120, sortable: true, filter: true },
    { field: 'due_date', headerName: 'Due Date', width: 120, sortable: true, filter: true },
    { field: 'currency', headerName: 'Currency', width: 100, sortable: true, filter: true },
    {
      field: 'amount', headerName: 'Amount', width: 140, sortable: true,
      valueFormatter: (params: ValueFormatterParams) => {
        if (params.value == null) return '';
        return Number(params.value).toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
      },
      cellStyle: { textAlign: 'right' }
    },
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
    this.lineItems = [];
    this.isEditMode = false;
    this.recalcTotals();
    this.onDateChange();
    this.activeTab = 'create';
  }

  editInvoice(inv: SalesInvoice) {
    this.formInvoice = { ...inv };
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
    this.lineItems = [];
  }

  // Line item management
  addLineItem() {
    this.lineItems.push({
      description: '',
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
      invoice_number: '',
      financial_year: this.financialYear(new Date().toISOString().split('T')[0]),
      business_partner_id: 0,
      invoice_date: new Date().toISOString().split('T')[0],
      due_in_days: 0,
      currency: 'INR',
      amount: 0,
      line_items: []
    };
  }

  downloadPDF(inv: SalesInvoice) {
    const items = inv.line_items || [];
    const isInr = inv.currency === 'INR';
    const subtotal = items.reduce((s, li) => s + li.amount, 0);
    const gstTotal = isInr ? items.reduce((s, li) => s + (li.amount * li.gst_percent / 100), 0) : 0;
    const grand = subtotal + gstTotal;

    const fmt = (n: number) => n.toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    const fmtDate = (d: string) => {
      try { return new Date(d).toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' }); }
      catch { return d; }
    };
    const esc = (s: string) => (s || '').replace(/\n/g, '<br/>');

    // Look up the business partner for full details
    const bp = this.businessPartners.find(p => p.id === inv.business_partner_id);
    const cp = this.companyProfile;

    const contact = bp?.contacts?.find(c => c.id === inv.contact_id);

    // Build line item rows
    let lineRows = '';
    items.forEach((li, i) => {
      const gstAmt = isInr ? li.amount * li.gst_percent / 100 : 0;
      lineRows += `<tr>
        <td style="text-align:center">${i + 1}</td>
        <td>${li.description}</td>
        ${isInr ? `<td>${li.hsn_sac_code || '-'}</td>` : ''}
        <td style="text-align:right">${li.quantity}</td>
        <td style="text-align:right">${fmt(li.rate)}</td>
        ${isInr ? `<td style="text-align:center">${li.gst_percent}%</td>` : ''}
        ${isInr ? `<td style="text-align:right">${fmt(gstAmt)}</td>` : ''}
        <td style="text-align:right">${fmt(li.amount + gstAmt)}</td>
      </tr>`;
    });

    const html = `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Invoice ${inv.invoice_number}</title>
  <style>
    @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: 'Inter', sans-serif; color: #1e293b; padding: 40px; background: #fff; }
    .invoice-container { max-width: 800px; margin: 0 auto; }
    .header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 32px; padding-bottom: 20px; border-bottom: 3px solid #6366f1; }
    .header-left h1 { font-size: 28px; color: #6366f1; font-weight: 700; letter-spacing: -0.5px; }
    .header-left .company-name { font-size: 15px; font-weight: 600; color: #1e293b; margin-top: 6px; }
    .header-left .company-detail { font-size: 12px; color: #64748b; margin-top: 2px; line-height: 1.5; }
    .header-right { text-align: right; }
    .header-right .inv-num { font-size: 18px; font-weight: 700; color: #1e293b; }
    .header-right .inv-meta { font-size: 13px; color: #64748b; margin-top: 4px; }
    .parties { display: flex; justify-content: space-between; margin-bottom: 28px; gap: 2rem; }
    .party-block { flex: 1; }
    .party-block h4 { font-size: 10px; text-transform: uppercase; letter-spacing: 1.2px; color: #94a3b8; margin-bottom: 8px; font-weight: 700; }
    .party-block .name { font-size: 14px; font-weight: 600; color: #1e293b; }
    .party-block .detail { font-size: 12px; color: #475569; line-height: 1.6; margin-top: 4px; }
    .party-block .tax-label { font-size: 11px; color: #94a3b8; font-weight: 600; margin-top: 6px; }
    .party-block .tax-value { font-size: 12px; color: #334155; font-weight: 500; }
    table { width: 100%; border-collapse: collapse; margin-bottom: 24px; }
    thead { background: #f1f5f9; }
    th { padding: 10px 12px; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: #64748b; font-weight: 600; text-align: left; border-bottom: 2px solid #e2e8f0; }
    td { padding: 10px 12px; font-size: 13px; border-bottom: 1px solid #f1f5f9; color: #334155; }
    tr:hover { background: #fafbfc; }
    .totals { display: flex; justify-content: flex-end; }
    .totals-table { width: 280px; }
    .totals-table tr td { border: none; padding: 6px 12px; }
    .totals-table .label { color: #64748b; font-weight: 500; }
    .totals-table .value { text-align: right; font-weight: 600; font-variant-numeric: tabular-nums; }
    .totals-table .grand { border-top: 2px solid #6366f1; }
    .totals-table .grand td { padding-top: 10px; font-size: 16px; font-weight: 700; }
    .totals-table .grand .value { color: #6366f1; }
    .footer { margin-top: 48px; padding-top: 20px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; }
    @media print { body { padding: 20px; } .no-print { display: none; } }
  </style>
</head>
<body>
  <div class="invoice-container">
    <div class="header">
      <div class="header-left">
        <h1>TAX INVOICE</h1>
        ${cp?.company_name ? `<div class="company-name">${cp.company_name}</div>` : ''}
        ${cp?.address ? `<div class="company-detail">${esc(cp.address)}</div>` : ''}
        ${cp?.email || cp?.phone ? `<div class="company-detail">${[cp?.email, cp?.phone].filter(Boolean).join(' | ')}</div>` : ''}
      </div>
      <div class="header-right">
        <div class="inv-num">${inv.invoice_number}</div>
        <div class="inv-meta">Date: ${fmtDate(inv.invoice_date)}</div>
        <div class="inv-meta">FY: ${inv.financial_year}</div>
        ${inv.due_date ? `<div class="inv-meta">Due Date: ${fmtDate(inv.due_date)}</div>` : ''}
      </div>
    </div>

    <div class="parties">
      <div class="party-block">
        <h4>From (Seller)</h4>
        ${cp?.company_name ? `<div class="name">${cp.company_name}</div>` : '<div class="name">-</div>'}
        ${cp?.address ? `<div class="detail">${esc(cp.address)}</div>` : ''}
        ${cp?.gstin ? `<div class="tax-label">GSTIN</div><div class="tax-value">${cp.gstin}</div>` : ''}
        ${cp?.pan ? `<div class="tax-label">PAN</div><div class="tax-value">${cp.pan}</div>` : ''}
      </div>
      <div class="party-block" style="text-align:right">
        <h4>Bill To (Buyer)</h4>
        <div class="name">${bp?.name || inv.business_partner_name || 'N/A'}</div>
        ${bp?.billing_address ? `<div class="detail">${esc(bp.billing_address)}</div>` : ''}
        ${bp?.tax_information ? `<div class="tax-label">Tax Info</div><div class="tax-value">${bp.tax_information}</div>` : ''}
        
        ${contact ? `
        <div style="margin-top: 12px; padding-top: 8px; border-top: 1px dashed #e2e8f0; display: inline-block; text-align: right;">
          <div class="detail" style="font-weight:600; color:#1e293b;">Attn: ${contact.name}</div>
          ${contact.email ? `<div class="detail" style="font-size:11px;">✉️ ${contact.email}</div>` : ''}
          ${contact.phone ? `<div class="detail" style="font-size:11px;">📞 ${contact.phone}</div>` : ''}
        </div>
        ` : ''}
      </div>
    </div>

    <table>
      <thead>
        <tr>
          <th style="width:40px;text-align:center">#</th>
          <th>Description</th>
          ${isInr ? '<th>HSN/SAC</th>' : ''}
          <th style="text-align:right">Qty</th>
          <th style="text-align:right">Rate</th>
          ${isInr ? '<th style="text-align:center">GST %</th>' : ''}
          ${isInr ? '<th style="text-align:right">GST Amt</th>' : ''}
          <th style="text-align:right">Total</th>
        </tr>
      </thead>
      <tbody>
        ${lineRows}
      </tbody>
    </table>

    <div class="totals">
      <table class="totals-table">
        <tr><td class="label">Subtotal</td><td class="value">${fmt(subtotal)}</td></tr>
        ${isInr && gstTotal > 0 ? `<tr><td class="label">GST</td><td class="value">${fmt(gstTotal)}</td></tr>` : ''}
        <tr class="grand"><td class="label">Total (${inv.currency})</td><td class="value">${fmt(grand)}</td></tr>
      </table>
    </div>

    <div class="footer">
      <p>This is a computer-generated invoice.</p>
    </div>
  </div>

  <div class="no-print" style="text-align:center; margin-top:32px;">
    <button onclick="window.print()" style="padding:12px 32px;background:#6366f1;color:#fff;border:none;border-radius:8px;font-size:14px;font-weight:600;cursor:pointer;font-family:inherit;">Print / Save as PDF</button>
  </div>
</body>
</html>`;

    const printWindow = window.open('', '_blank');
    if (printWindow) {
      printWindow.document.write(html);
      printWindow.document.close();
    }
  }
}
