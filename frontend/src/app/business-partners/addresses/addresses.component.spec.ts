import { PartnerAddressesComponent } from './addresses.component';

describe('Partner address drafts', () => {
  it('keeps stored address data unchanged until the parent saves', () => {
    const component = new PartnerAddressesComponent();
    component.addresses = [{id: 1, address: 'Original', is_archived: false}];
    spyOn(component.addressesChange, 'emit');
    component.change(0, {address: 'Revised'});
    expect(component.addresses[0].address).toBe('Original');
    expect(component.addressesChange.emit).toHaveBeenCalledWith([{id: 1, address: 'Revised', is_archived: false}]);
  });
  it('adds an independent address without a previous-version ID', () => {
    const component = new PartnerAddressesComponent(); spyOn(component.addressesChange, 'emit'); component.add();
    expect(component.addressesChange.emit).toHaveBeenCalledWith([{address: '', is_archived: false}]);
  });
});
