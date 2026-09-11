import { of, Subject } from 'rxjs';
import { GSTR3BComponent } from './gstr3b.component';
import { ApiService, GSTR3BSummary } from '../api.service';

describe('GSTR3BComponent', () => {
  it('loads the selected year and preserves missing conversion values', () => {
    const api = jasmine.createSpyObj<ApiService>('ApiService', ['getGSTR3B', 'downloadGSTR3B']);
    api.getGSTR3B.and.returnValue(of({ financial_year: '2026-2027', financial_years: ['2025-2026'], rows: [{
      invoice_number: '1', currency: 'USD', amount: 20, invoiced_to: 'Client', address: 'Address',
      invoice_date: '2026-04-01', exchange_rate: null, value_inr: null, reference_date: null
    }] }));
    const component = new GSTR3BComponent(api);
    component.year = '2026-2027'; component.load();
    expect(api.getGSTR3B).toHaveBeenCalledWith('2026-2027');
    expect(component.years).toEqual(['2026-2027', '2025-2026']);
    expect(component.missingRates).toBe(1);
    expect(component.rows[0].value_inr).toBeNull();
    component.ngOnDestroy();
  });
  it('clears stale rows and recovers after a failed request', () => {
    const api = jasmine.createSpyObj<ApiService>('ApiService', ['getGSTR3B', 'downloadGSTR3B']);
    const response = new Subject<GSTR3BSummary>(); api.getGSTR3B.and.returnValue(response);
    const component = new GSTR3BComponent(api); component.year = '2026-2027'; component.load();
    expect(component.loading).toBeTrue();
    component.download(); expect(api.downloadGSTR3B).not.toHaveBeenCalled();
    response.error(new Error('Offline'));
    expect(component.loading).toBeFalse(); expect(component.error).toContain('Could not load');
    api.getGSTR3B.and.returnValue(of({financial_year: component.year, financial_years: [], rows: []}));
    component.load(); expect(component.error).toBe(''); expect(component.rows).toEqual([]);
    component.ngOnDestroy();
  });
});
