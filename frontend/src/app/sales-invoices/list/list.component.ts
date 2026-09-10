import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, SalesInvoice, BusinessPartner } from '../../api.service';
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
  currencies: string[] = ['USD', 'GBP', 'EUR', 'AED', 'INR', 'CHF'];
  isLoading = true;
  isSaving = false;
  isDarkMode = true;

  activeTab: 'view' | 'create' = 'view';
  isEditMode = false;

  formInvoice: SalesInvoice = this.emptyInvoice();

  // AG Grid Column Definitions
  public columnDefs: ColDef[] = [
    { field: 'invoice_number', headerName: 'Invoice #', flex: 1, sortable: true, filter: true },
    { field: 'financial_year', headerName: 'FY', width: 110, sortable: true, filter: true },
    { field: 'business_partner_name', headerName: 'Business Partner', flex: 1.5, sortable: true, filter: true },
    { field: 'invoice_date', headerName: 'Date', width: 130, sortable: true, filter: true },
    { field: 'currency', headerName: 'Currency', width: 110, sortable: true, filter: true },
    {
      field: 'amount', headerName: 'Amount', width: 140, sortable: true,
      valueFormatter: (params: ValueFormatterParams) => {
        if (params.value == null) return '';
        return Number(params.value).toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
      },
      cellStyle: { textAlign: 'right' }
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
    }
  ];

  public defaultColDef: ColDef = {
    resizable: true,
  };

  constructor(private api: ApiService) {}

  ngOnInit() {
    // Detect theme
    this.isDarkMode = !document.body.hasAttribute('data-theme') ||
                      document.body.getAttribute('data-theme') !== 'light';

    const observer = new MutationObserver(() => {
      this.isDarkMode = !document.body.hasAttribute('data-theme') ||
                        document.body.getAttribute('data-theme') !== 'light';
    });
    observer.observe(document.body, { attributes: true, attributeFilter: ['data-theme'] });

    this.loadData();
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
    this.isEditMode = false;
    this.activeTab = 'create';
  }

  editInvoice(inv: SalesInvoice) {
    this.formInvoice = { ...inv };
    this.isEditMode = true;
    this.activeTab = 'create';
  }

  cancelForm() {
    this.activeTab = 'view';
    this.isEditMode = false;
    this.formInvoice = this.emptyInvoice();
  }

  save() {
    const inv = this.formInvoice;
    if (!inv.invoice_number || !inv.financial_year || !inv.business_partner_id ||
        !inv.invoice_date || !inv.currency || inv.amount === null || inv.amount === undefined) {
      alert('All fields are mandatory. Please fill out all details.');
      return;
    }

    this.isSaving = true;

    if (this.isEditMode && inv.id) {
      this.api.updateSalesInvoice(inv.id, inv).subscribe({
        next: () => {
          this.isSaving = false;
          this.activeTab = 'view';
          this.loadData();
        },
        error: (err) => {
          alert('Failed to update: ' + (err.error?.message || err.message));
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
          alert('Failed to create: ' + (err.error?.message || err.message));
          this.isSaving = false;
        }
      });
    }
  }

  private emptyInvoice(): SalesInvoice {
    return {
      invoice_number: '',
      financial_year: '2026-27',
      business_partner_id: 0,
      invoice_date: new Date().toISOString().split('T')[0],
      currency: 'INR',
      amount: 0
    };
  }
}
