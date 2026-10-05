// Aero "Streamline A": the letter A drawn only from wind-tunnel smoke lines
// rising over a peak, crossed by the orange streamline files ride on.
//
// Usage: node scripts/brand/mark.cjs [lines]   → prints JSON { lines: [...], crossbar }
// All geometry lives in a 64×64 box.

const f = (n) => +n.toFixed(2);

/**
 * One streamline of the A. `k` is 0 for the outer line and grows inward:
 * inner lines sit lower and narrower, like nested smoke filaments.
 */
function strand(k) {
  const apexY = 8 + k * 7; // peak height
  const footX = 5 + k * 6.2; // where the line settles at the base
  const baseY = 55;
  const shoulder = 0.42; // how steep the climb is (0 = vertical, 1 = flat)
  const up = [
    [footX, baseY],
    // settle horizontally at the foot, like air leaving the body
    [footX + 6.5, baseY],
    [32 - (32 - footX) * shoulder * 0.55, apexY + 9],
    [32, apexY],
  ];
  const down = up.map(([x, y]) => [64 - x, y]).reverse();
  return (
    `M${f(up[0][0])} ${f(up[0][1])}` +
    `C${f(up[1][0])} ${f(up[1][1])} ${f(up[2][0])} ${f(up[2][1])} ${f(up[3][0])} ${f(up[3][1])}` +
    `C${f(down[1][0])} ${f(down[1][1])} ${f(down[2][0])} ${f(down[2][1])} ${f(down[3][0])} ${f(down[3][1])}`
  );
}

/** The orange crossbar: a streamline that lifts slightly as it crosses the A. */
const crossbar = 'M-4 41.2C12 41.2 22 38.8 32 38.8S52 41.2 68 41.2';

function mark(count = 3) {
  return { lines: Array.from({ length: count }, (_, k) => strand(k)), crossbar };
}

module.exports = { mark, strand, crossbar };
if (require.main === module) console.log(JSON.stringify(mark(+(process.argv[2] || 3)), null, 2));
