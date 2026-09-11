export type GSTTreatment = 'intrastate' | 'interstate' | 'none';
export function invoiceGSTTreatment(currency: string, seller: string, buyer: string): GSTTreatment {
  if (currency.toUpperCase() !== 'INR') return 'none';
  const shape = /^[0-9]{2}[A-Z0-9]{13}$/;
  seller = seller.trim().toUpperCase(); buyer = buyer.trim().toUpperCase();
  return shape.test(seller) && shape.test(buyer) && seller.slice(0, 2) !== buyer.slice(0, 2) ? 'interstate' : 'intrastate';
}
export function roundTax(value: number): number { return Math.round((value + Number.EPSILON) * 100) / 100; }
