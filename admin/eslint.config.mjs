import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
  {
    rules: {
      // Setiap halaman admin memuat data lewat `useEffect(() => { load(); }, [])`
      // dan `load()` menyalakan state loading lebih dulu. Pola itu disengaja dan
      // benar secara React; aturan ini berasal dari panduan migrasi React
      // Compiler (menyarankan use()/Server Components/React Query), bukan
      // indikasi bug. Dimatikan agar `--max-warnings=0` di CI tetap berguna
      // untuk aturan lain. Bisa dinyalakan lagi kalau data-fetching dipindah ke
      // React Query atau Server Components.
      "react-hooks/set-state-in-effect": "off",
    },
  },
]);

export default eslintConfig;
