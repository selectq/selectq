import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { BPAddress } from '../../api.service';

@Component({
  selector: 'app-partner-addresses', standalone: true, imports: [CommonModule, FormsModule],
  template: `
    <div class="heading"><h3>Billing addresses</h3><button type="button" (click)="add()" [disabled]="disabled">+ Add address</button></div>
    <p>Saving an edited address creates a new version and archives the previous version. Existing invoices retain their address.</p>
    <p *ngIf="!addresses.length">No addresses added.</p>
    <div class="address" *ngFor="let address of addresses; let i = index">
      <label [for]="'bp-address-' + i">{{ address.id ? 'Address #' + address.id : 'New address' }} {{ address.is_archived ? '(archived)' : '' }}</label>
      <small *ngIf="address.previous_address_id">Replaces #{{ address.previous_address_id }}</small>
      <textarea [id]="'bp-address-' + i" rows="3" [ngModel]="address.address" [ngModelOptions]="{standalone: true}" (ngModelChange)="change(i, {address: $event})" [disabled]="disabled || !!address.is_archived" placeholder="Full billing address"></textarea>
      <label *ngIf="address.id && !originallyArchived(address)"><input type="checkbox" [ngModel]="address.is_archived" [ngModelOptions]="{standalone: true}" (ngModelChange)="change(i, {is_archived: $event})" [disabled]="disabled" /> Archive on save</label>
      <button type="button" *ngIf="!address.id" (click)="remove(i)" [disabled]="disabled">Remove</button>
    </div>`,
  styles: [`:host {display:block;margin:1rem 0} .heading{display:flex;justify-content:space-between;align-items:center;gap:1rem} p,small{color:var(--text-secondary);font-size:.85rem;margin:.6rem 0;display:block} .address{border:1px solid var(--surface-border);padding:1rem;border-radius:8px;margin:.8rem 0} textarea{display:block;width:100%;margin:.5rem 0;padding:.6rem;background:var(--input-bg);color:var(--text-primary);border:1px solid var(--surface-border);border-radius:6px;font:inherit} button{padding:.45rem .7rem;border:1px solid var(--surface-border);border-radius:6px;background:var(--input-bg);color:var(--text-primary);cursor:pointer} input[type=checkbox]{width:auto} label{display:block} textarea:disabled{opacity:.7}`]
})
export class PartnerAddressesComponent {
  @Input() addresses: BPAddress[] = [];
  @Input() disabled = false;
  @Output() addressesChange = new EventEmitter<BPAddress[]>();
  // Original archive state is supplied by the parent for persisted records.
  originallyArchived(address: BPAddress) { return address.was_archived ?? false; }
  add() { this.addressesChange.emit([...this.addresses, { address: '', is_archived: false }]); }
  remove(index: number) { this.addressesChange.emit(this.addresses.filter((_, i) => i !== index)); }
  change(index: number, patch: Partial<BPAddress>) { this.addressesChange.emit(this.addresses.map((a, i) => i === index ? { ...a, ...patch } : a)); }
}
