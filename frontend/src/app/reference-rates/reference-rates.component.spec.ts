import { ComponentFixture, TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiService, ReferenceRate } from '../api.service';
import { ReferenceRatesComponent } from './reference-rates.component';

describe('Reference rates page', () => {
  let fixture: ComponentFixture<ReferenceRatesComponent>;
  let component: ReferenceRatesComponent;
  let api: jasmine.SpyObj<ApiService>;
  const saved: ReferenceRate = { rate_date: '2026-08-31', currency: 'JPY', units: 100, rate_inr: 59.72, source_file: 'ReferenceRate.xlsx' };
  beforeEach(async () => {
    api = jasmine.createSpyObj('ApiService', ['getReferenceRates', 'createReferenceRate', 'updateReferenceRate', 'uploadReferenceRates']);
    api.getReferenceRates.and.returnValue(of([{ ...saved }]));
    api.createReferenceRate.and.returnValue(of(saved));
    api.updateReferenceRate.and.returnValue(of(saved));
    api.uploadReferenceRates.and.returnValue(of({ inserted: 6, skipped: 0, empty_skipped: 2, sheet: 'August Rates', from_date: '2026-08-31', to_date: '2026-08-31' }));
    await TestBed.configureTestingModule({ imports: [ReferenceRatesComponent], providers: [{ provide: ApiService, useValue: api }] }).compileComponents();
    fixture = TestBed.createComponent(ReferenceRatesComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
    await fixture.whenStable();
  });
  afterEach(() => fixture.destroy());
  it('loads the grid and saves a manual quote with the chosen currency units', async () => {
    expect(component.rates[0]).toEqual(saved);
    expect(fixture.nativeElement.querySelector('ag-grid-angular')).toBeTruthy();
    (fixture.nativeElement.querySelector('.page-header button') as HTMLButtonElement).click();
    fixture.detectChanges(); await fixture.whenStable();
    const currency = fixture.nativeElement.querySelector('#rate-currency') as HTMLInputElement;
    currency.value = 'idr'; currency.dispatchEvent(new Event('input'));
    fixture.detectChanges(); await fixture.whenStable();
    expect(component.draft?.currency).toBe('IDR');
    expect(component.draft?.units).toBe(10000);
    component.draft!.rate_inr = 53.7816;
    component.draft!.rate_date = '2026-08-31';
    fixture.detectChanges(); await fixture.whenStable();
    (fixture.nativeElement.querySelector('form') as HTMLFormElement).dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    expect(api.createReferenceRate).toHaveBeenCalledWith({ rate_date: '2026-08-31', currency: 'IDR', units: 10000, rate_inr: 53.7816 });
    expect(component.draft).toBeNull();
    expect(api.getReferenceRates).toHaveBeenCalledTimes(2);
  });
  it('preserves the original grid row and unsaved edits on a conflict', () => {
    api.updateReferenceRate.and.returnValue(throwError(() => ({ error: 'A reference rate already exists.' })));
    component.edit(component.rates[0]);
    component.draft!.rate_date = '2026-08-28'; component.draft!.rate_inr = 60;
    component.save(); fixture.detectChanges();
    expect(api.updateReferenceRate).toHaveBeenCalledWith(saved, { rate_date: '2026-08-28', currency: 'JPY', units: 100, rate_inr: 60 });
    expect(component.rates[0]).toEqual(saved);
    expect(component.draft?.rate_inr).toBe(60);
    expect(fixture.nativeElement.querySelector('[role="alert"]').textContent).toContain('already exists');
    component.cancel(); expect(component.draft).toBeNull();
  });
  it('imports the selected workbook and refreshes the grid with an import summary', () => {
    const file = new File(['test'], 'ReferenceRate.xlsx');
    component.file = file;
    component.upload(fixture.nativeElement.querySelector('#rates-file'));
    fixture.detectChanges();
    expect(api.uploadReferenceRates).toHaveBeenCalledWith(file, '');
    expect(api.getReferenceRates).toHaveBeenCalledTimes(2);
    expect(component.file).toBeNull();
    const summary = fixture.nativeElement.querySelector('[role="status"]').textContent;
    expect(summary).toContain('Imported 6 rates');
    expect(summary).toContain('2 empty cells');
    expect(summary).toContain('August Rates');
  });
  it('allows a worksheet override and resets it for the next workbook', () => {
    const file = new File(['test'], 'ReferenceRate.xlsx');
    component.file = file; component.sheet = ' Second Rates ';
    component.upload(fixture.nativeElement.querySelector('#rates-file'));
    expect(api.uploadReferenceRates).toHaveBeenCalledWith(file, 'Second Rates');
    component.chooseFile({ target: { files: [new File(['test'], 'next.xlsx')] } } as unknown as Event);
    expect(component.sheet).toBe('');
  });
  it('blocks invalid manual values and retains the file after an upload error', () => {
    component.add(); component.draft!.units = 1.5; component.draft!.rate_inr = 95;
    component.save(); expect(api.createReferenceRate).not.toHaveBeenCalled();
    component.cancel();
    api.uploadReferenceRates.and.returnValue(throwError(() => ({ error: 'Invalid date in row 4' })));
    const file = new File(['test'], 'rates.xlsx'); component.file = file;
    component.upload(fixture.nativeElement.querySelector('#rates-file')); fixture.detectChanges();
    expect(component.file).toBe(file); expect(component.importing).toBeFalse();
    expect(component.rates[0]).toEqual(saved);
    expect(fixture.nativeElement.querySelector('[role="alert"]').textContent).toContain('row 4');
  });
});
