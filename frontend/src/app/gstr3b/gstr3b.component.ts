import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { AgGridAngular } from 'ag-grid-angular';
import { ColDef } from 'ag-grid-community';
import { Subscription } from 'rxjs';
import { ApiService, GSTR3BRow } from '../api.service';

@Component({
  selector: 'app-gstr3b', standalone: true, imports: [CommonModule, FormsModule, AgGridAngular],
  templateUrl: './gstr3b.component.html', styleUrl: './gstr3b.component.css'
})
export class GSTR3BComponent implements OnInit, OnDestroy {
  rows: GSTR3BRow[] = [];
  years: string[] = [];
  year = '';
  loading = false;
  downloading = false;
  error = '';
  isDarkMode = true;
  private observer?: MutationObserver;
  defaultColDef: ColDef<GSTR3BRow> = {
    sortable: true, filter: true, resizable: true, wrapHeaderText: true, autoHeaderHeight: true
  };
  columnDefs: ColDef<GSTR3BRow>[] = [
    { field: 'invoice_number', headerName: 'Invoice Number', initialSort: 'asc', width: 165 },
    { field: 'currency', headerName: 'Currency', width: 115 },
    { field: 'amount', headerName: 'Amount in Currency', width: 175, filter: 'agNumberColumnFilter',
      cellStyle: { textAlign: 'right' }, valueFormatter: p => this.formatAmount(p.value) },
    { field: 'invoiced_to', headerName: 'Invoiced To', minWidth: 190, flex: 1 },
    { field: 'address', headerName: 'Address', minWidth: 240, flex: 2, wrapText: true, autoHeight: true,
      cellStyle: { whiteSpace: 'pre-wrap', lineHeight: '1.5', paddingTop: '8px', paddingBottom: '8px' } },
    { field: 'invoice_date', headerName: 'Invoice Date', width: 140 },
    { field: 'exchange_rate', headerName: 'Exchange Rate (INR per unit)', width: 185, filter: 'agNumberColumnFilter',
      cellStyle: { textAlign: 'right' }, valueFormatter: p => p.value == null ? 'Missing rate' :
        Number(p.value).toLocaleString('en-IN', { maximumFractionDigits: 12 }) },
    { field: 'value_inr', headerName: 'Value in INR', width: 170, filter: 'agNumberColumnFilter',
      cellStyle: { textAlign: 'right' }, valueFormatter: p => this.formatAmount(p.value) },
    { field: 'reference_date', headerName: 'Reference Date', width: 155,
      valueFormatter: p => p.data?.currency === 'INR' ? 'Not applicable' : (p.value || 'Unavailable') }
  ];
  private formatAmount(value: number | null | undefined): string {
    return value == null ? '—' : value.toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  private requests = new Subscription();
  constructor(private api: ApiService) {}
  ngOnInit() {
    const updateTheme = () => this.isDarkMode = document.body.getAttribute('data-theme') !== 'light';
    updateTheme();
    this.observer = new MutationObserver(updateTheme);
    this.observer.observe(document.body, { attributes: true, attributeFilter: ['data-theme'] });
    const now = new Date();
    const start = now.getFullYear() - (now.getMonth() < 3 ? 1 : 0);
    this.year = `${start}-${start + 1}`;
    this.load();
  }
  ngOnDestroy() { this.observer?.disconnect(); this.requests.unsubscribe(); }
  get missingRates() { return this.rows.filter(row => row.exchange_rate === null).length; }
  load() {
    this.loading = true; this.error = ''; this.rows = [];
    this.requests.add(this.api.getGSTR3B(this.year).subscribe({
      next: summary => {
        this.rows = summary.rows;
        this.years = [...new Set([this.year, ...summary.financial_years])].sort().reverse();
        this.loading = false;
      },
      error: () => { this.error = 'Could not load the GSTR-3B summary. Please try again.'; this.loading = false; }
    }));
  }
  download() {
    if (this.loading || this.downloading || !this.year) return;
    const year = this.year;
    this.downloading = true; this.error = '';
    this.requests.add(this.api.downloadGSTR3B(year).subscribe({
      next: blob => {
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url; link.download = `GSTR3B-${year}.xlsx`;
        document.body.appendChild(link); link.click(); link.remove();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
        this.downloading = false;
      },
      error: () => { this.error = 'Could not download the workbook. Please try again.'; this.downloading = false; }
    }));
  }
}
