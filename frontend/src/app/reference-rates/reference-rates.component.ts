import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { AgGridAngular } from 'ag-grid-angular';
import { ColDef } from 'ag-grid-community';
import { Subscription } from 'rxjs';
import { ApiService, ReferenceRate } from '../api.service';

@Component({
  selector: 'app-reference-rates', standalone: true,
  imports: [CommonModule, FormsModule, AgGridAngular],
  templateUrl: './reference-rates.component.html',
  styleUrl: './reference-rates.component.css'
})
export class ReferenceRatesComponent implements OnInit, OnDestroy {
  rates: ReferenceRate[] = [];
  fromDate = ''; toDate = ''; downloading = false;
  get validRange() { return !!this.fromDate && !!this.toDate && this.fromDate <= this.toDate; }
  download() {
    if (this.busy || this.downloading || !this.validRange) return;
    const from = this.fromDate, to = this.toDate;
    this.downloading = true; this.error = '';
    this.requests.add(this.api.downloadReferenceRates(from, to).subscribe({
      next: blob => {
        if (blob.type !== 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet') {
          this.downloading = false;
          this.error = 'The server did not return an Excel workbook. Restart the updated server and try again.';
          return;
        }
        const url = URL.createObjectURL(blob); const a = document.createElement('a');
        a.href = url; a.download = `ReferenceRates-${from}-to-${to}.xlsx`;
        document.body.appendChild(a); a.click(); a.remove();
        setTimeout(() => URL.revokeObjectURL(url), 1000); this.downloading = false;
      },
      error: err => { this.downloading = false; this.error = err.status === 404 ? 'No reference rates found in the selected date range.' : 'Could not download rates. Check the date range and try again.'; }
    }));
  }
  file: File | null = null;
  sheet = '';
  draft: ReferenceRate | null = null;
  original: ReferenceRate | null = null;
  loading = false;
  saving = false;
  importing = false;
  error = '';
  message = '';
  isDarkMode = true;
  private requests = new Subscription();
  private observer?: MutationObserver;
  get busy() { return this.loading || this.saving || this.importing; }
  get validDraft() {
    const r = this.draft;
    return !!r && /^\d{4}-\d{2}-\d{2}$/.test(r.rate_date) && /^[A-Z]{3}$/.test(r.currency)
      && r.currency !== 'INR' && Number.isSafeInteger(r.units) && r.units > 0
      && Number.isFinite(r.rate_inr) && r.rate_inr > 0;
  }
  defaultColDef: ColDef<ReferenceRate> = { sortable: true, filter: true, resizable: true };
  columnDefs: ColDef<ReferenceRate>[] = [
    { field: 'rate_date', headerName: 'Date', width: 130, initialSort: 'desc', sortIndex: 0 },
    { field: 'currency', headerName: 'Currency', width: 115, initialSort: 'asc', sortIndex: 1 },
    { field: 'units', headerName: 'Quote units', width: 135, filter: 'agNumberColumnFilter' },
    { field: 'rate_inr', headerName: 'Rate (INR per quote)', width: 195, filter: 'agNumberColumnFilter',
      valueFormatter: p => p.value == null ? '' : Number(p.value).toLocaleString('en-IN', { minimumFractionDigits: 4, maximumFractionDigits: 12 }),
      cellStyle: { textAlign: 'right' } },
    { field: 'source_file', headerName: 'Source', flex: 1, minWidth: 190 },
    { field: 'source_sheet', headerName: 'Worksheet', width: 135 },
    { field: 'imported_at', headerName: 'Saved at (UTC)', width: 185 },
    { colId: 'edit', headerName: '', width: 85, sortable: false, filter: false,
      cellRenderer: (p: { data?: ReferenceRate }) => {
        const button = document.createElement('button');
        button.textContent = 'Edit';
        button.className = 'grid-edit-btn';
        button.type = 'button';
        button.setAttribute('aria-label', `Edit ${p.data?.currency} rate for ${p.data?.rate_date}`);
        button.addEventListener('click', () => { if (p.data) this.edit(p.data); });
        return button;
      } }
  ];
  constructor(private api: ApiService) {}
  ngOnInit() {
    const updateTheme = () => this.isDarkMode = document.body.getAttribute('data-theme') !== 'light';
    updateTheme();
    this.observer = new MutationObserver(updateTheme);
    this.observer.observe(document.body, { attributes: true, attributeFilter: ['data-theme'] });
    this.load();
  }
  ngOnDestroy() { this.observer?.disconnect(); this.requests.unsubscribe(); }
  load() {
    this.loading = true;
    this.requests.add(this.api.getReferenceRates().subscribe({
      next: rates => { this.rates = rates || []; this.loading = false; },
      error: err => { this.error = this.errorText(err); this.loading = false; }
    }));
  }
  refresh() { if (!this.busy) { this.error = ''; this.load(); } }
  chooseFile(event: Event) {
    this.file = (event.target as HTMLInputElement).files?.[0] || null;
    this.sheet = '';
    this.error = ''; this.message = '';
  }
  upload(input: HTMLInputElement) {
    if (this.busy || !this.file || this.draft) return;
    if (!this.file.name.toLowerCase().endsWith('.xlsx') || this.file.size > 9 * 1024 * 1024) {
      this.error = 'Choose an .xlsx workbook up to 9 MB.'; return;
    }
    this.importing = true; this.error = ''; this.message = '';
    this.requests.add(this.api.uploadReferenceRates(this.file, this.sheet.trim()).subscribe({
      next: result => {
        this.importing = false;
        this.message = `Imported ${result.inserted} rates; skipped ${result.skipped} identical rates and ${result.empty_skipped} empty cells from worksheet "${result.sheet}" (${result.from_date} to ${result.to_date}).`;
        this.file = null; input.value = ''; this.load();
      },
      error: err => { this.importing = false; this.error = this.errorText(err); }
    }));
  }
  add() {
    if (this.busy) return;
    const today = new Date();
    const date = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;
    this.original = null;
    this.draft = { rate_date: date, currency: 'USD', units: 1, rate_inr: 0 };
    this.error = ''; this.message = '';
  }
  edit(rate: ReferenceRate) {
    if (this.busy) return;
    this.original = { ...rate }; this.draft = { ...rate };
    this.error = ''; this.message = '';
  }
  currencyChanged(currency: string) {
    if (!this.draft) return;
    this.draft.currency = currency.trim().toUpperCase();
    if (!this.original) this.draft.units = this.draft.currency === 'JPY' ? 100 : this.draft.currency === 'IDR' ? 10000 : 1;
  }
  cancel() { if (!this.busy) { this.draft = null; this.original = null; this.error = ''; } }
  save() {
    if (this.busy || !this.validDraft || !this.draft) return;
    const { rate_date, currency, units, rate_inr } = this.draft;
    const payload = { rate_date, currency, units, rate_inr };
    const request = this.original ? this.api.updateReferenceRate(this.original, payload) : this.api.createReferenceRate(payload);
    this.saving = true; this.error = ''; this.message = '';
    this.requests.add(request.subscribe({
      next: () => { this.saving = false; this.draft = null; this.original = null; this.message = 'Reference rate saved.'; this.load(); },
      error: err => { this.saving = false; this.error = this.errorText(err); }
    }));
  }
  private errorText(err: any): string {
    return typeof err?.error === 'string' ? err.error.trim() : 'Could not complete the request. Please try again.';
  }
}
