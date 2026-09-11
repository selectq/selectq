import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subscription } from 'rxjs';
import { ApiService, GSTR3BRow } from '../api.service';

@Component({
  selector: 'app-gstr3b', standalone: true, imports: [CommonModule, FormsModule],
  templateUrl: './gstr3b.component.html', styleUrl: './gstr3b.component.css'
})
export class GSTR3BComponent implements OnInit, OnDestroy {
  rows: GSTR3BRow[] = [];
  years: string[] = [];
  year = '';
  loading = false;
  downloading = false;
  error = '';
  private requests = new Subscription();
  constructor(private api: ApiService) {}
  ngOnInit() {
    const now = new Date();
    const start = now.getFullYear() - (now.getMonth() < 3 ? 1 : 0);
    this.year = `${start}-${start + 1}`;
    this.load();
  }
  ngOnDestroy() { this.requests.unsubscribe(); }
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
