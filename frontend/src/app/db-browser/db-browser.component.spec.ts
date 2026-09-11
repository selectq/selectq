import { ComponentFixture, TestBed } from '@angular/core/testing';
import { of, Subject, throwError } from 'rxjs';
import { ApiService, DBObject, DBResult } from '../api.service';
import { DBBrowserComponent } from './db-browser.component';

describe('DBBrowserComponent', () => {
  let fixture: ComponentFixture<DBBrowserComponent>;
  let component: DBBrowserComponent;
  let api: jasmine.SpyObj<ApiService>;
  const table: DBObject = { name: 'accounts', type: 'table', table_name: 'accounts', sql: 'CREATE TABLE accounts(account_no TEXT PRIMARY KEY)' };
  const result: DBResult = { columns: ['account_no'], rows: [['123']], truncated: false, changes: 0 };
  beforeEach(async () => {
    api = jasmine.createSpyObj('ApiService', ['getDBObjects', 'getDBSettings', 'getDBRows', 'getDBDetail', 'executeSQL']);
    api.getDBObjects.and.returnValue(of([table]));
    api.getDBSettings.and.returnValue(of({ foreign_keys: { ...result, columns: ['foreign_keys'], rows: [[1]] } }));
    api.getDBRows.and.returnValue(of(result));
    api.getDBDetail.and.returnValue(of({ object: table, sections: {} }));
    api.executeSQL.and.returnValue(of(result));
    await TestBed.configureTestingModule({ imports: [DBBrowserComponent], providers: [{ provide: ApiService, useValue: api }] }).compileComponents();
    fixture = TestBed.createComponent(DBBrowserComponent); component = fixture.componentInstance; fixture.detectChanges();
  });
  it('renders the object list and selected rows', () => {
    component.selectObject(table); fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('123');
    expect(fixture.nativeElement.textContent).toContain('Structure & constraints');
  });
  it('pages rows and exposes structure', () => {
    component.selectObject(table); component.page(1);
    expect(api.getDBRows).toHaveBeenCalledWith('accounts', 100);
    component.changeTab('structure'); fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('CREATE TABLE accounts');
  });
  it('cancels stale row requests when opening a setting', () => {
    const pending = new Subject<DBResult>(); api.getDBRows.and.returnValue(pending);
    component.selectObject(table); component.selectSetting('foreign_keys'); pending.next(result);
    expect(component.view).toBe('settings'); expect(component.result).toBeNull(); expect(component.loading).toBeFalse();
  });
  it('preserves SQL and displays backend errors', () => {
    api.executeSQL.and.returnValue(throwError(() => ({ error: 'no such table: missing' })));
    component.openSQL(); component.sql = 'SELECT * FROM missing'; component.runSQL(); fixture.detectChanges();
    expect(component.sql).toBe('SELECT * FROM missing');
    expect(fixture.nativeElement.textContent).toContain('no such table: missing'); expect(component.running).toBeFalse();
  });
  it('quotes identifiers when opening a table in the editor', () => {
    component.selected = { ...table, name: 'a"b' }; component.querySelected();
    expect(component.sql).toBe('SELECT * FROM "a""b" LIMIT 100;');
  });
});
