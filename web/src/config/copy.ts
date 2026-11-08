import { SITE_CONFIG } from "./site";

export const SITE_COPY = {
  announcements: [
    `Ambil langsung di ${SITE_CONFIG.location.address}`,
    "Pesan mudah via WhatsApp",
    "Garansi sesuai ketentuan resmi brand",
  ],
  hero: {
    label: "GLOBAL MANDIRI COMPUTER · SURABAYA",
    headlineLines: ["Rakit.", "Upgrade.", "Terhubung."],
    subline:
      "Kurasi perangkat keras komputer, laptop, komponen rakitan, infrastruktur jaringan, dan Starlink di ITC Mega Grosir Surabaya.",
    ctaLabel: "Belanja sekarang",
    ctaHref: "/shop",
  },
  whyGmc: {
    label: "STANDAR LAYANAN KAMI",
    heading: "Dirancang untuk keandalan jangka panjang.",
    pillars: [
      {
        id: "original",
        tabLabel: "BARANG ORIGINAL",
        title: "Produk Original & Distribusi Resmi",
        body: "Seluruh unit perangkat keras, laptop, periferal, dan perangkat jaringan yang kami sediakan berasal dari jalur distribusi resmi masing-masing brand di Indonesia.",
        caption: "STUDIO HARDWARE · GARANSI RESMI",
      },
      {
        id: "stok-itc",
        tabLabel: "STOK SIAP DI ITC",
        title: `Tersedia Langsung di ${SITE_CONFIG.location.address}`,
        body: "Kunjungi showroom fisik kami di lantai 2 ITC Mega Grosir Surabaya untuk melihat unit secara langsung, mengambil pesanan hari yang sama, atau berdiskusi dengan teknisi kami.",
        caption: "SHOWROOM FISIK · SURABAYA",
      },
      {
        id: "konsultasi",
        tabLabel: "KONSULTASI RAKIT PC",
        title: "Konsultasi Kompatibilitas & Perakitan Presisi",
        body: "Mulai dari pemilihan soket prosesor, profil memori DDR5, efisiensi daya PSU ATX 3.1, hingga manajemen aliran udara casing—tim kami memastikan setiap komponen bekerja selaras.",
        caption: "WORKSHOP RAKITAN · PRESISI TERMAL",
      },
      {
        id: "harga",
        tabLabel: "HARGA BERSAING",
        title: "Harga Transparan dalam Rupiah",
        body: "Harga tertera secara jelas untuk pembelian satuan maupun pengadaan perangkat kantor dan proyek, tanpa biaya tersembunyi.",
        caption: "TRANSPARAN · RETAIL & KORPORASI",
      },
      {
        id: "purna-jual",
        tabLabel: "LAYANAN PURNA JUAL",
        title: "Pendampingan Purna Jual yang Responsif",
        body: "Kami membantu pemeriksaan awal kendala perangkat dan mengarahkan proses klaim garansi sesuai ketentuan resmi distributor masing-masing brand.",
        caption: "DUKUNGAN TEKNIS · WHATSAPP & TOKO",
      },
    ],
  },
  about: {
    heroLabel: "TENTANG GLOBAL MANDIRI COMPUTER",
    heroTitle: "Partner perangkat keras & konektivitas Anda di Surabaya.",
    storyTitle: "Berangkat dari kebutuhan nyata akan perangkat yang tepat.",
    storyParagraphs: [
      "Global Mandiri Computer (GMC) hadir di ITC Mega Grosir Lt. 2 Blok B/7, Surabaya untuk menjawab kebutuhan pengguna rumahan, profesional kreatif, gamer, hingga tim IT perusahaan yang membutuhkan perangkat komputer dan jaringan berkualitas.",
      "Kami percaya bahwa membeli komponen PC, laptop, proyektor, maupun perangkat Starlink seharusnya menjadi pengalaman yang tenang, jelas, dan terarah. Karena itu kami mengutamakan kejelasan spesifikasi, kurasi brand tepercaya, serta komunikasi langsung tanpa perantara rumit.",
    ],
    processSteps: [
      {
        step: "01",
        title: "Konsultasi Kebutuhan",
        description:
          "Diskusikan alur kerja, anggaran, dan spesifikasi teknis Anda bersama tim kami via WhatsApp atau langsung di toko.",
      },
      {
        step: "02",
        title: "Rekomendasi & Validasi",
        description:
          "Kami menyusun daftar komponen atau unit yang kompatibel, mengecek ketersediaan stok fisik, dan merinci estimasi biaya.",
      },
      {
        step: "03",
        title: "Perakitan & Pengujian",
        description:
          "Untuk PC rakitan, unit dirakit dengan manajemen kabel rapi, pembaruan BIOS, dan pengujian beban kerja dasar.",
      },
      {
        step: "04",
        title: "Ambil di Toko atau Kirim Aman",
        description:
          "Ambil pesanan Anda langsung di ITC Mega Grosir Lt. 2 Blok B/7 atau kirim ke alamat tujuan dengan pelindung ekstra.",
      },
    ],
  },
  footer: {
    newsletterLabel: "Tetap terhubung untuk info produk & promo terbaru",
    newsletterStep1Placeholder: "Nama depan Anda",
    newsletterStep2Placeholder: "Alamat email Anda",
  },
} as const;
