# Aero brand

Aero is named after aerodynamics. The mark is the **Streamline A**: the letter A
drawn only from wind-tunnel smoke filaments rising over a peak, crossed by an
orange streamline: the one your files ride on.

| File | Purpose |
|------|---------|
| `mark.cjs` | Source of truth for the mark's geometry (64×64 box). Prints JSON. |
| `icons.cjs` | Renders the app icon at every size (fewer, bolder strands when small) and writes `assets/icon.ico`, `assets/appicon.png` and `assets/appicon.svg`. |

After changing `mark.cjs`:

```sh
# 1. Update the in-app geometry
node -e "const {mark}=require('./scripts/brand/mark.cjs');console.log(mark(3))"
#    → copy into frontend/src/components/brand/geometry.ts and the phone page header
# 2. Re-render the icons (drives a local Chrome)
npm i --no-save puppeteer-core
node scripts/brand/icons.cjs assets
```

| Token | Value | Use |
|-------|-------|-----|
| Graphite | `#0B0D10` | Background, the wind tunnel |
| Smoke | `#E8ECF0` | Streamlines, text |
| Signal (aviation orange) | `#FF5B1F` | The crossbar streamline, live transfers, primary actions |

Type: Archivo (expanded width for the wordmark and headlines) and JetBrains Mono for telemetry.
