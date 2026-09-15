import type { ScreenRow } from "./api";

// Kamus istilah: kata teknis -> penjelasan santai 1 kalimat.
export const ISTILAH: Record<string, string> = {
  "akumulasi": "Broker-broker besar lagi borong saham ini diam-diam. Biasanya tanda mereka tahu sesuatu yang bagus.",
  "distribusi": "Broker-broker besar lagi jualan saham ini pelan-pelan. Hati-hati, bisa turun.",
  "foreign flow": "Uang investor luar negeri yang masuk (beli) atau keluar (jual) dari saham ini.",
  "inflow": "Uang asing lagi masuk — mereka beli. Tanda kepercayaan.",
  "outflow": "Uang asing lagi keluar — mereka jual. Waspada.",
  "broker": "Perantara jual-beli saham (sekuritas). Gerakan mereka dibaca buat nebak arah pasar.",
  "net": "Selisih total beli dikurangi total jual. Positif = lebih banyak yang beli.",
  "composite": "Nilai gabungan dari semua sinyal (asing + broker + volume). Makin tinggi makin menarik.",
  "screener": "Alat penyaring: cari saham yang memenuhi kriteria tertentu.",
  "briefing": "Ringkasan pagi otomatis: kabar pasar hari ini dalam beberapa kalimat.",
  "alert": "Notifikasi otomatis kalau saham incaranmu bergerak aneh.",
  "routine": "Jadwal otomatis, misalnya terima ringkasan pasar tiap pagi jam 7.",
  "portofolio": "Kumpulan semua saham yang kamu pegang.",
  "diversifikasi": "Jangan taruh semua uang di satu saham — sebar biar aman.",
  "rekomendasi": "Saran beli/tahan/jual dari sistem. Bukan nasihat keuangan resmi — tetap riset sendiri.",
  "hold": "Tahan: jangan jual dulu, tapi belum saatnya tambah beli.",
  "buy": "Beli: sinyalnya bagus, layak dipertimbangkan.",
  "sell": "Jual: sinyalnya jelek, pertimbangkan keluar.",
  "citations": "Sumber data: dari API mana angka ini diambil, biar bisa diverifikasi.",
  "live": "Data real-time dari scheduler, bukan contoh.",
  "stale": "Data sudah lama, belum diperbarui. Jangan terlalu dipercaya.",
  "mover": "Saham yang harganya bergerak drastis hari ini.",
  "agenda": "Jadwal penting emiten: bagi dividen, rapat pemegang saham, laporan keuangan.",
  "ex-div": "Tanggal batas: kalau beli setelah tanggal ini, tidak dapat dividen periode ini.",
};

export type Verdict = "Dilirik" | "Dilepas" | "Netral";

export function verdictFor(row: ScreenRow): { verdict: Verdict; alasan: string } {
  const b = (row.breakdown || {}) as Record<string, unknown>;
  const comp = Number(row.composite ?? 0);
  const trend = String(b.foreign_trend ?? "");
  const broker = Number(b.broker_score ?? b.broker ?? 0);
  const insider = Boolean(b.insider_buying ?? b.insider);
  if (comp > 5 || (trend === "inflow" && broker > 0)) {
    const bits: string[] = [];
    if (trend === "inflow") bits.push("uang asing masuk");
    if (broker > 0) bits.push("broker besar borong");
    if (insider) bits.push("orang dalam ikut beli");
    return { verdict: "Dilirik", alasan: bits.length ? bits.join(", ") : "sinyal gabungan positif" };
  }
  if (comp < -5 || (trend === "outflow" && broker < 0)) {
    const bits: string[] = [];
    if (trend === "outflow") bits.push("uang asing keluar");
    if (broker < 0) bits.push("broker besar jualan");
    return { verdict: "Dilepas", alasan: bits.length ? bits.join(", ") : "sinyal gabungan negatif" };
  }
  return { verdict: "Netral", alasan: "belum ada gerakan mencolok — pantau saja" };
}

// Ringkas briefing mentah jadi 3 kalimat santai.
export function heroSummary(briefing: string): string[] {
  const lines = briefing.split("\n").filter((l) => l.trim());
  const top = lines.filter((l) => /^\d+\./.test(l.trim())).slice(0, 3);
  const foreign = lines.filter((l) => /^-/.test(l.trim()) && /Rp/i.test(l)).slice(0, 2);
  const mover = lines.find((l) => /mover/i.test(l));
  const out: string[] = [];
  if (top.length) {
    const names = top.map((t) => t.replace(/^\d+\.\s*/, "").split(" net")[0]).join(", ");
    out.push(`Pagi ini yang paling diborong: ${names}. Artinya pemain besar lagi kumpul di sana.`);
  }
  if (foreign.length) out.push(`Uang asing: ${foreign.map((f) => f.replace(/^-\s*/, "")).join("; ")}.`);
  if (mover) out.push(`Perhatian khusus: ${mover.replace(/^.*?:\s*/, "")}.`);
  if (!out.length && briefing.trim()) out.push(briefing.slice(0, 220));
  return out.slice(0, 3);
}
