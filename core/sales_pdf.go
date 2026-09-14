package core

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/coregx/gxpdf/builder"
)

//go:embed assets/select-q-wordmark.ttf
var selectQWordmark []byte

// SalesInvoicePDF generates an invoice followed by the fixed annexure supplied
// in the Studio South reference document. Overflow invoices paginate before it.
func SalesInvoicePDF(inv SalesInvoice, company CompanyProfile, contact BusinessPartnerContact) ([]byte, error) {
	populateInvoiceGST(&inv)
	doc := builder.NewBuilder(builder.WithPageSize(builder.A4),
		builder.WithMargins(builder.Mm(18), builder.Mm(16), builder.Mm(18), builder.Mm(16)),
		builder.WithTitle("Invoice "+inv.InvoiceNumber), builder.WithDefaultFontSize(10), builder.WithFont("SelectQLogo", selectQWordmark))
	text := func(c *builder.Container, value string) {
		for _, line := range strings.Split(strings.ReplaceAll(value, "\r", ""), "\n") {
			c.Text(line)
		}
	}
	field := func(c *builder.Container, label, value string) {
		if value != "" {
			text(c, label+": "+value)
		}
	}
	doc.Page(func(p *builder.PageBuilder) {
		invoiceLetterhead(p)
		p.Content(func(c *builder.Container) {
			c.Text("Invoice: "+inv.InvoiceNumber, builder.Bold(), builder.FontSize(16))
			c.Spacer(builder.Mm(4))
			c.Row(func(r *builder.RowBuilder) {
				r.Col(7, func(col *builder.ColBuilder) {
					col.Text("Billed To", builder.Bold())
					text(&col.Container, inv.BusinessPartnerName)
					text(&col.Container, inv.BillingAddress)
					field(&col.Container, "Customer Name", contact.Name)
					text(&col.Container, strings.TrimSpace(contact.Email+" "+contact.Phone))
					field(&col.Container, "Buyer GSTIN", inv.BuyerGSTIN)
				})
				r.Col(5, func(col *builder.ColBuilder) {
					col.Text("From", builder.Bold())
					text(&col.Container, company.CompanyName)
					text(&col.Container, company.Address)
					text(&col.Container, strings.TrimSpace(company.Email+" "+company.Phone))
					field(&col.Container, "Supplier GSTIN", inv.SellerGSTIN)
					field(&col.Container, "PAN", company.PAN)
				})
			})
			c.Spacer(builder.Mm(5))
			c.Row(func(r *builder.RowBuilder) {
				r.Col(7, func(col *builder.ColBuilder) {
					field(&col.Container, "Invoice Reference No", inv.InvoiceNumber)
					field(&col.Container, "Invoice Date", inv.InvoiceDate)
					field(&col.Container, "Payment Terms", fmt.Sprintf("%d Days", inv.DueInDays))
					field(&col.Container, "Due Date", inv.DueDate)
				})
				r.Col(5, func(col *builder.ColBuilder) {
					field(&col.Container, "Project Name", inv.ProjectName)
					field(&col.Container, "Our Ref", inv.OurReference)
					field(&col.Container, "Your Ref", inv.YourReference)
					field(&col.Container, "Order No", inv.OrderNumber)
				})
			})
			c.Spacer(builder.Mm(6))
			inr := strings.EqualFold(inv.Currency, "INR")
			c.Table(func(t *builder.TableBuilder) {
				widths := []builder.Value{builder.Fr(5), builder.Fr(1), builder.Fr(1.5), builder.Fr(1.7)}
				headers := []string{"Description", "Qty", "Unit Price (" + inv.Currency + ")", "Amount (" + inv.Currency + ")"}
				if inr {
					widths = append(widths, builder.Fr(2.5))
					headers = append(headers, "HSN/SAC; GST %")
				}
				t.Columns(widths...)
				row := func(r *builder.TableRowBuilder, values []string, bold bool) {
					for _, v := range values {
						r.Cell(func(cell *builder.CellBuilder) {
							if bold {
								cell.Text(v, builder.Bold(), builder.FontSize(9))
							} else {
								text(&cell.Container, v)
							}
						}, builder.CellPadding(builder.Mm(2)), builder.CellBorder(builder.Hex("cccccc"), 0.5))
					}
				}
				t.Header(func(r *builder.TableRowBuilder) { row(r, headers, true) })
				for _, li := range inv.LineItems {
					values := []string{li.Description, fmt.Sprintf("%g", li.Quantity), fmt.Sprintf("%.2f", li.Rate), fmt.Sprintf("%.2f", li.Amount)}
					if inr {
						values = append(values, fmt.Sprintf("%s\nCGST %g%%\nSGST %g%%\nIGST %g%%", li.HsnSacCode, li.CGSTPercent, li.SGSTPercent, li.IGSTPercent))
					}
					t.Row(func(r *builder.TableRowBuilder) { row(r, values, false) })
				}
			})
			c.KeepTogether(func(c *builder.Container) {
				c.Spacer(builder.Mm(3))
				c.Text(fmt.Sprintf("Subtotal: %s %.2f", inv.Currency, inv.Amount), builder.AlignRight())
				if inr {
					if inv.GSTTreatment == "interstate" {
						c.Text(fmt.Sprintf("IGST: %.2f", inv.IGSTAmount), builder.AlignRight())
					} else {
						c.Text(fmt.Sprintf("CGST: %.2f", inv.CGSTAmount), builder.AlignRight())
						c.Text(fmt.Sprintf("SGST: %.2f", inv.SGSTAmount), builder.AlignRight())
					}
				}
				c.Text(fmt.Sprintf("Total: %s %.2f", inv.Currency, inv.Amount+inv.GSTAmount), builder.AlignRight(), builder.Bold(), builder.FontSize(12))
			})
			c.Spacer(builder.Mm(7))
			c.Text("Additional Information", builder.Bold())
			c.Text("Payment must be done in " + inv.Currency + " & transfer charges must be borne by Payer")
			text(c, inv.AdditionalInformation)
			c.Text("Annexure - I - Bank Account Information")
		})

	})
	doc.Page(func(p *builder.PageBuilder) {
		invoiceLetterhead(p)
		p.Content(func(c *builder.Container) {
			c.Text("ANNEXURE - I: BENEFICIARY ACCOUNT INFORMATION", builder.Bold(), builder.FontSize(14))
			c.Spacer(builder.Mm(12))
			c.Table(func(t *builder.TableBuilder) {
				t.Columns(builder.Fr(1), builder.Fr(2))
				rows := []struct {
					label, value string
					heading      bool
				}{
					{"Beneficiary:", "", true},
					{"Account Name", "SELECT Q", false},
					{"Account Number", "50200020525472", false},
					{"Account Address", "RITU GANDHI\nC/O SELECT Q\nHOUSE NO 189, SECTOR 31\nFARIDABAD, AMARNAGAR", false},
					{"Beneficiary Bank Details", "", true},
					{"Beneficiary Bank Name", "HDFC Bank Ltd", false},
					{"Swift Code", "HDFCINBBDEL", false},
					{"Bank / Branch Address", "HDFC BANK, SECTOR 31, FARIDABAD\nCITY: FARIDABAD\nSTATE: HARYANA\nZIP: 121003\nCOUNTRY: INDIA", false},
				}
				for _, row := range rows {
					t.Row(func(r *builder.TableRowBuilder) {
						for _, value := range []string{row.label, row.value} {
							r.Cell(func(cell *builder.CellBuilder) {
								if row.heading {
									cell.Text(value, builder.Bold())
								} else {
									text(&cell.Container, value)
								}
							}, builder.CellPadding(builder.Mm(2)), builder.CellBorder(builder.Hex("cccccc"), 0.5))
						}
					})
				}
			})
		})
	})
	return doc.Build()
}

// invoiceLetterhead reproduces the fixed header/footer in the supplied Word template.
func invoiceLetterhead(p *builder.PageBuilder) {
	rule := func(c *builder.Container) { c.Line(builder.LineColor(builder.Hex("5b9bd5")), builder.LineWidth(0.5)) }
	p.Header(func(c *builder.Container) {
		c.Text("\ue000", builder.FontFamily("SelectQLogo"), builder.FontSize(20), builder.LineHeight(1), builder.TextColor(builder.Hex("4472c4")))
		c.Text("Registered Office 189, Sector 31, Faridabad", builder.FontSize(8))
		c.Spacer(builder.Mm(2))
		rule(c)
		c.Spacer(builder.Mm(5))
	})
	p.Footer(func(c *builder.Container) {
		rule(c)
		c.Spacer(builder.Mm(3))
		c.Row(func(r *builder.RowBuilder) {
			r.Col(6, func(col *builder.ColBuilder) {
				col.RichText(func(t *builder.RichTextBuilder) {
					t.Span("GSTIN:  ", builder.Bold())
					t.Span("06AKLPM9722C2Z3")
				}, builder.FontSize(8))
				col.RichText(func(t *builder.RichTextBuilder) {
					t.Span("E-Mail:  ", builder.Bold())
					t.Span("ritugandhi@selectq.in")
				}, builder.FontSize(8))
			})
			r.Col(2, func(col *builder.ColBuilder) { col.Text("Registered Address", builder.Bold(), builder.FontSize(8)) })
			r.Col(4, func(col *builder.ColBuilder) {
				for _, line := range []string{"RITU GANDHI", "C/O SELECT Q", "House No 189, Sector 31, Faridabad", "AMARNAGAR,", "CITY FARIDABAD STATE HARYANA", "ZIP 121003, INDIA"} {
					col.Text(line, builder.FontSize(8))
				}
			})
		})
	})
}
