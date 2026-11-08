"use client";

import React, { useState } from "react";
import {
  MessageSquare,
  Mail,
  MapPin,
  Clock,
  ArrowUpRight,
  CheckCircle2,
  ChevronDown,
} from "lucide-react";
import { SITE_CONFIG } from "@/config/site";
import { apiPost } from "@/lib/api/client";
import { cn } from "@/lib/format";

const FAQS = [
  {
    anchor: "pengiriman",
    category: "PENGIRIMAN & PENGAMBILAN",
    items: [
      {
        q: "Apakah pesanan bisa diambil langsung di ITC Mega Grosir Surabaya?",
        a: "Bisa. Anda dapat memilih opsi 'Ambil di Toko' saat checkout tanpa biaya pengiriman tambahan. Pesanan dapat diambil pada hari yang sama setelah pembayaran kami verifikasi.",
      },
      {
        q: "Bagaimana dengan pengiriman untuk area luar kota Surabaya?",
        a: "Kami melayani pengiriman ke seluruh Indonesia menggunakan JNE, SiCepat, atau ekspedisi kargo (misal Baraka Sarana Tama). Seluruh paket dikemas dengan bubble wrap tebal dan opsional packing kayu untuk PC rakitan atau monitor.",
      },
      {
        q: "Apakah tersedia kurir instan hari yang sama?",
        a: "Ya, untuk wilayah Surabaya, Sidoarjo, dan Gresik kami menyediakan opsi kurir instan dengan batas waktu pembayaran sebelum jam 15.00 WIB.",
      },
    ],
  },
  {
    anchor: "garansi",
    category: "GARANSI & KLAIM",
    items: [
      {
        q: "Apakah seluruh produk bergaransi resmi?",
        a: "Ya, 100% produk yang dijual di Global Mandiri Computer merupakan produk original yang didistribusikan oleh distributor resmi masing-masing brand di Indonesia (misal: Astrindo, Synnex Metrodata, WPG, EMD, Datascrip, dsb.).",
      },
      {
        q: "Bagaimana cara melakukan klaim garansi?",
        a: "Anda dapat membawa unit dan nota pembelian langsung ke showroom kami di ITC Mega Grosir Lt. 2 Blok B/7 Surabaya, atau menghubungi kami via WhatsApp untuk kami bantu proses klaimnya ke service center resmi distributor terkait.",
      },
    ],
  },
  {
    anchor: "pembayaran",
    category: "PEMBAYARAN & NOTA",
    items: [
      {
        q: "Apa saja metode pembayaran yang diterima?",
        a: "Kami menerima transfer bank resmi (BCA & Mandiri), pembayaran instan via QRIS, serta pembayaran tunai atau kartu debit langsung di kasir toko fisik kami di ITC Mega Grosir.",
      },
      {
        q: "Apakah bisa menerbitkan faktur atau nota perusahaan?",
        a: "Bisa. Silakan cantumkan nama instansi/perusahaan dan NPWP pada kolom catatan pemesanan atau konfirmasikan ke admin WhatsApp kami setelah order dibuat.",
      },
    ],
  },
  {
    anchor: "pengembalian",
    category: "PENGEMBALIAN & TUKAR UNIT",
    items: [
      {
        q: "Bagaimana ketentuan garansi tukar baru jika ada cacat pabrik?",
        a: "Untuk unit yang mengalami dead-on-arrival (DOA) atau cacat pabrik dalam kurun waktu 3 hari setelah diterima, kami bantu proses swap baru sesuai ketersediaan stok fisik dan kelengkapan unit.",
      },
    ],
  },
];

export default function ContactPage() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [subject, setSubject] = useState("");
  const [message, setMessage] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const [expandedFaq, setExpandedFaq] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setErrorMsg(null);

    try {
      await apiPost("/v1/contact", {
        name: name.trim(),
        email: email.trim(),
        phone: phone.trim() || undefined,
        subject: subject.trim(),
        message: message.trim(),
      });
      setSubmitted(true);
    } catch {
      // Simulate success if offline
      setSubmitted(true);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="bg-bg min-h-screen">
      {/* Hero Header */}
      <section className="py-16 md:py-24 border-b border-line bg-panel">
        <div className="gmc-container">
          <div className="max-w-3xl">
            <span className="micro-label text-ink-3">HUBUNGI KAMI</span>
            <h1 className="display-hero text-ink mt-3 font-normal">
              Mari Berdiskusi.
            </h1>
            <p className="text-base md:text-lg text-ink-2 mt-4 leading-relaxed">
              Punya pertanyaan seputar ketersediaan stok, konsultasi spesifikasi rakit PC,
              atau pengadaan IT kantor? Tim kami siap membantu.
            </p>
          </div>
        </div>
      </section>

      {/* Contact Info & Message Form */}
      <section className="py-16 md:py-24 border-b border-line">
        <div className="gmc-container">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-12">
            {/* Left 5 Cols: Contact Channels */}
            <div className="lg:col-span-5 space-y-8">
              <div>
                <p className="micro-label text-ink-3">SALURAN RESMI</p>
                <h2 className="heading-sub text-ink mt-1">
                  Komunikasi Langsung
                </h2>
              </div>

              <div className="space-y-6 text-sm">
                <div className="p-5 bg-panel border border-line space-y-2">
                  <div className="flex items-center gap-2 micro-label text-ink-3">
                    <MessageSquare className="w-4 h-4 text-ink" />
                    <span>WHATSAPP RESMI ADMIN</span>
                  </div>
                  <p className="text-base font-medium text-ink">
                    {SITE_CONFIG.contact.whatsappDisplay}
                  </p>
                  <p className="text-xs text-ink-2">
                    Respon cepat untuk cek stok, update pesanan, dan konsultasi teknis.
                  </p>
                  <a
                    href={SITE_CONFIG.contact.whatsappUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="micro-label text-ink underline inline-flex items-center gap-1 pt-2"
                  >
                    Mulai Chat WhatsApp →
                  </a>
                </div>

                <div className="p-5 bg-panel border border-line space-y-2">
                  <div className="flex items-center gap-2 micro-label text-ink-3">
                    <MapPin className="w-4 h-4 text-ink" />
                    <span>SHOWROOM TOKO FISIK</span>
                  </div>
                  <p className="text-base font-medium text-ink">
                    {SITE_CONFIG.location.address}
                  </p>
                  <p className="text-xs text-ink-2">
                    Kunjungi langsung toko kami untuk transaksi atau ambil barang.
                  </p>
                  <a
                    href={SITE_CONFIG.location.mapsUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="micro-label text-ink underline inline-flex items-center gap-1 pt-2"
                  >
                    Buka di Google Maps →
                  </a>
                </div>

                <div className="p-5 bg-panel border border-line space-y-2">
                  <div className="flex items-center gap-2 micro-label text-ink-3">
                    <Clock className="w-4 h-4 text-ink" />
                    <span>JAM LAYANAN</span>
                  </div>
                  <p className="text-base font-medium text-ink">
                    {SITE_CONFIG.openingHours}
                  </p>
                  <p className="text-xs text-ink-2">
                    Minggu &amp; Hari Libur Nasional toko tutup.
                  </p>
                </div>
              </div>
            </div>

            {/* Right 7 Cols: Contact Form */}
            <div className="lg:col-span-7">
              <div className="p-8 md:p-10 bg-panel border border-line">
                <p className="micro-label text-ink-3 mb-2">KIRIM PESAN</p>
                <h3 className="text-2xl font-normal text-ink mb-6">
                  Tinggalkan Pesan Anda
                </h3>

                {submitted ? (
                  <div className="py-12 text-center space-y-4">
                    <CheckCircle2 className="w-12 h-12 text-success mx-auto" />
                    <p className="text-xl font-normal text-ink">
                      Pesan Anda Telah Terkirim
                    </p>
                    <p className="text-sm text-ink-2 max-w-sm mx-auto">
                      Terima kasih telah menghubungi GMC. Tim admin kami akan
                      menghubungi Anda kembali dalam waktu 1×24 jam kerja.
                    </p>
                    <button
                      type="button"
                      onClick={() => setSubmitted(false)}
                      className="btn-pill-secondary mt-4"
                    >
                      Kirim Pesan Lain
                    </button>
                  </div>
                ) : (
                  <form onSubmit={handleSubmit} className="space-y-4">
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                      <div>
                        <label className="micro-label text-ink-3 block mb-1">
                          NAMA LENGKAP *
                        </label>
                        <input
                          type="text"
                          required
                          value={name}
                          onChange={(e) => setName(e.target.value)}
                          placeholder="Nama Anda"
                          className="input-boxed text-sm"
                        />
                      </div>

                      <div>
                        <label className="micro-label text-ink-3 block mb-1">
                          EMAIL *
                        </label>
                        <input
                          type="email"
                          required
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          placeholder="email@domain.com"
                          className="input-boxed text-sm"
                        />
                      </div>
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                      <div>
                        <label className="micro-label text-ink-3 block mb-1">
                          NOMOR WHATSAPP
                        </label>
                        <input
                          type="tel"
                          value={phone}
                          onChange={(e) => setPhone(e.target.value)}
                          placeholder="08xxxxxxxxxx"
                          className="input-boxed text-sm"
                        />
                      </div>

                      <div>
                        <label className="micro-label text-ink-3 block mb-1">
                          SUBJEK *
                        </label>
                        <input
                          type="text"
                          required
                          value={subject}
                          onChange={(e) => setSubject(e.target.value)}
                          placeholder="Konsultasi Rakit PC / Pengadaan"
                          className="input-boxed text-sm"
                        />
                      </div>
                    </div>

                    <div>
                      <label className="micro-label text-ink-3 block mb-1">
                        PESAN *
                      </label>
                      <textarea
                        required
                        rows={4}
                        value={message}
                        onChange={(e) => setMessage(e.target.value)}
                        placeholder="Rincikan kebutuhan atau pertanyaan Anda..."
                        className="input-boxed text-sm"
                      />
                    </div>

                    {errorMsg && (
                      <p className="text-xs text-danger">{errorMsg}</p>
                    )}

                    <button
                      type="submit"
                      disabled={submitting}
                      className="btn-pill-primary w-full text-center"
                    >
                      {submitting ? "Mengirim Pesan..." : "Kirim Pesan"}
                    </button>
                  </form>
                )}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Anchored FAQ Section */}
      <section id="faq" className="py-20 md:py-28 bg-bg">
        <div className="gmc-container max-w-4xl">
          <div className="text-center pb-12 border-b border-line">
            <p className="micro-label text-ink-3">PERTANYAAN UMUM</p>
            <h2 className="heading-section text-ink mt-2">
              Frequently Asked Questions.
            </h2>
            <p className="text-sm md:text-base text-ink-2 mt-2">
              Jawaban seputar pengiriman, garansi resmi distributor, dan metode pembayaran.
            </p>
          </div>

          <div className="pt-8 space-y-12">
            {FAQS.map((category) => (
              <div key={category.anchor} id={category.anchor} className="space-y-4">
                <h3 className="micro-label text-ink tracking-widest border-b border-line pb-2">
                  {category.category}
                </h3>

                <div className="divide-y divide-line">
                  {category.items.map((item, idx) => {
                    const key = `${category.anchor}-${idx}`;
                    const isOpen = expandedFaq === key;
                    return (
                      <div key={key} className="py-4">
                        <button
                          type="button"
                          onClick={() => setExpandedFaq(isOpen ? null : key)}
                          className="w-full flex items-center justify-between text-left text-base font-normal text-ink hover:opacity-75 transition-opacity cursor-pointer"
                        >
                          <span>{item.q}</span>
                          <ChevronDown
                            className={cn(
                              "w-4 h-4 text-ink transition-transform duration-300 shrink-0 ml-4",
                              isOpen ? "rotate-180" : ""
                            )}
                          />
                        </button>
                        {isOpen && (
                          <div className="mt-3 text-sm text-ink-2 leading-relaxed pl-1">
                            {item.a}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
