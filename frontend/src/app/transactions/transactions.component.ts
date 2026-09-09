import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, BankTransaction, BusinessPartner } from '../api.service';
import { ActivatedRoute, Router } from '@angular/router';
import { AgGridAngular } from 'ag-grid-angular';
import { ColDef, CellValueChangedEvent, ValueSetterParams, ValueFormatterParams } from 'ag-grid-community';

@Component({
  selector: 'app-transactions',
  standalone: true,
  imports: [CommonModule, FormsModule, AgGridAngular],
  templateUrl: './transactions.component.html',
  styleUrls: ['./transactions.component.css']
})
export class TransactionsComponent implements OnInit {
  transactions: BankTransaction[] = [];
  businessPartners: BusinessPartner[] = [];
  isLoading = true;
  importId: number | null = null;
  isDarkMode = false;

  public columnDefs: ColDef[] = [
    { field: 'date', headerName: 'Date', width: 120, sortable: true, filter: true },
    { field: 'narration', headerName: 'Narration', flex: 2, sortable: true, filter: true, wrapText: true, autoHeight: true },
    { field: 'withdrawal_amt', headerName: 'Withdrawal', width: 130, sortable: true, valueFormatter: this.currencyFormatter },
    { field: 'deposit_amt', headerName: 'Deposit', width: 130, sortable: true, valueFormatter: this.currencyFormatter },
    { field: 'invoice_number', headerName: 'Invoice Number', editable: true, flex: 1, sortable: true, filter: true },
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
        // Revert value? For now just alert.
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
