import { Routes } from '@angular/router';
import { UploadComponent } from './upload/upload.component';
import { HistoryComponent } from './history/history.component';
import { TransactionsComponent } from './transactions/transactions.component';
import { ListComponent } from './business-partners/list/list.component';
import { FormComponent } from './business-partners/form/form.component';

export const routes: Routes = [
  { path: '', redirectTo: 'upload', pathMatch: 'full' },
  { path: 'upload', component: UploadComponent },
  { path: 'history', component: HistoryComponent },
  { path: 'history/:id', component: TransactionsComponent },
  { path: 'business-partners', component: ListComponent },
  { path: 'business-partners/new', component: FormComponent },
  { path: 'business-partners/edit/:id', component: FormComponent }
];
