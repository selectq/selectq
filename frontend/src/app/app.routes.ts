import { Routes } from '@angular/router';
import { GSTR3BComponent } from './gstr3b/gstr3b.component';
import { UploadComponent } from './upload/upload.component';
import { HistoryComponent } from './history/history.component';
import { TransactionsComponent } from './transactions/transactions.component';
import { ListComponent } from './business-partners/list/list.component';

import { ListComponent as SalesInvoicesList } from './sales-invoices/list/list.component';
import { SettingsComponent } from './settings/settings.component';

import { DBBrowserComponent } from './db-browser/db-browser.component';

import { ReferenceRatesComponent } from './reference-rates/reference-rates.component';

export const routes: Routes = [
  { path: '', redirectTo: 'upload', pathMatch: 'full' },
  { path: 'upload', component: UploadComponent },
  { path: 'history', component: HistoryComponent },
  { path: 'history/:id', component: TransactionsComponent },
  { path: 'business-partners', component: ListComponent },
  { path: 'sales-invoices', component: SalesInvoicesList },
  { path: 'gstr3b', component: GSTR3BComponent },
  { path: 'db-browser', component: DBBrowserComponent },
  { path: 'reference-rates', component: ReferenceRatesComponent },
  { path: 'settings', component: SettingsComponent }
];
