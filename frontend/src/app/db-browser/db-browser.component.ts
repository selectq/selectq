import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subscription } from 'rxjs';
import { ApiService, DBObject, DBObjectDetail, DBResult } from '../api.service';

@Component({
  selector: 'app-db-browser', standalone: true, imports: [CommonModule, FormsModule],
  templateUrl: './db-browser.component.html', styleUrls: ['./db-browser.component.css']
})
export class DBBrowserComponent implements OnInit, OnDestroy {
  objects: DBObject[] = [];
  settings: Record<string, DBResult> = {};
  filter = '';
  selected: DBObject | null = null;
  detail: DBObjectDetail | null = null;
  result: DBResult | null = null;
  sqlResult: DBResult | null = null;
  setting = '';
  view: 'object' | 'settings' | 'sql' = 'object';
  tab: 'rows' | 'structure' = 'rows';
  offset = 0;
  loading = false;
  running = false;
  error = '';
  catalogError = '';
  sqlError = '';
  sql = 'SELECT name, type, sql FROM sqlite_schema ORDER BY type, name;';
  mode = 'query';
  private selectionRequest = new Subscription();
  private requests = new Subscription();
  constructor(private api: ApiService) {}
  ngOnInit() { this.refreshCatalog(); }
  ngOnDestroy() { this.selectionRequest.unsubscribe(); this.requests.unsubscribe(); }
  get groups() {
    const matches = (o: DBObject) => o.name.toLowerCase().includes(this.filter.toLowerCase());
    return [
      { label: 'Tables', objects: this.objects.filter(o => o.type === 'table' && matches(o)) },
      { label: 'Views', objects: this.objects.filter(o => o.type === 'view' && matches(o)) },
      { label: 'Indexes', objects: this.objects.filter(o => o.type === 'index' && matches(o)) },
      { label: 'Sequences', objects: this.objects.filter(o => ['sqlite_sequence', 'invoice_sequences'].includes(o.name) && matches(o)) },
      { label: 'Triggers', objects: this.objects.filter(o => o.type === 'trigger' && matches(o)) }
    ];
  }
  get settingNames() { return Object.keys(this.settings).sort().filter(n => n.includes(this.filter.toLowerCase())); }
  get canBrowse() { return this.selected?.type === 'table' || this.selected?.type === 'view'; }
  refreshCatalog() {
    this.catalogError = '';
    this.requests.add(this.api.getDBObjects().subscribe({ next: objects => {
      this.objects = objects;
      if (this.selected && !objects.some(o => o.name === this.selected!.name)) { this.selected = null; this.detail = null; this.result = null; }
    }, error: err => this.catalogError = this.message(err) }));
    this.requests.add(this.api.getDBSettings().subscribe({ next: settings => this.settings = settings, error: err => this.catalogError = this.message(err) }));
  }
  private resetSelection() { this.selectionRequest.unsubscribe(); this.selectionRequest = new Subscription(); this.error = ''; this.loading = false; }
  selectObject(object: DBObject) {
    this.resetSelection(); this.selected = object; this.view = 'object'; this.detail = null; this.result = null; this.offset = 0;
    this.tab = this.canBrowse ? 'rows' : 'structure'; this.loadSelection();
  }
  loadSelection() {
    if (!this.selected) return;
    this.resetSelection(); this.loading = true;
    if (this.tab === 'rows' && this.canBrowse) {
      this.selectionRequest.add(this.api.getDBRows(this.selected.name, this.offset).subscribe({ next: result => { this.result = result; this.loading = false; }, error: err => { this.error = this.message(err); this.loading = false; } }));
    } else {
      this.selectionRequest.add(this.api.getDBDetail(this.selected.name).subscribe({ next: detail => { this.detail = detail; this.loading = false; }, error: err => { this.error = this.message(err); this.loading = false; } }));
    }
  }
  changeTab(tab: 'rows' | 'structure') { this.tab = tab; this.loadSelection(); }
  page(delta: number) { this.offset = Math.max(0, this.offset + delta * 100); this.loadSelection(); }
  selectSetting(name: string) { this.resetSelection(); this.setting = name; this.view = 'settings'; }
  openSQL() { this.resetSelection(); this.view = 'sql'; }
  querySelected() {
    if (!this.selected) return;
    this.sql = `SELECT * FROM "${this.selected.name.replace(/"/g, '""')}" LIMIT 100;`; this.mode = 'query'; this.openSQL();
  }
  runSQL() {
    if (this.running || !this.sql.trim()) return;
    this.running = true; this.sqlError = ''; this.sqlResult = null;
    this.requests.add(this.api.executeSQL(this.sql, this.mode).subscribe({
      next: result => { this.sqlResult = result; this.running = false; this.refreshCatalog(); },
      error: err => { this.sqlError = this.message(err); this.running = false; this.refreshCatalog(); }
    }));
  }
  cell(value: unknown): string { return value === null ? 'NULL' : String(value); }
  private message(err: any): string { return typeof err.error === 'string' ? err.error : err.message || 'Request failed'; }
}
