/**
 * A tiny hand-rolled icon set so the app ships no icon-font dependency and no
 * network requests. Every shape is expressed as SVG path data on a 24x24 grid
 * with a 1.7px stroke.
 */

const circle = (cx: number, cy: number, r: number): string =>
  `M ${cx - r} ${cy} a ${r} ${r} 0 1 0 ${2 * r} 0 a ${r} ${r} 0 1 0 ${-2 * r} 0`

const roundedRect = (x: number, y: number, w: number, h: number, r = 2): string =>
  `M ${x + r} ${y} h ${w - 2 * r} a ${r} ${r} 0 0 1 ${r} ${r} v ${h - 2 * r} ` +
  `a ${r} ${r} 0 0 1 ${-r} ${r} h ${-(w - 2 * r)} a ${r} ${r} 0 0 1 ${-r} ${-r} ` +
  `v ${-(h - 2 * r)} a ${r} ${r} 0 0 1 ${r} ${-r} z`

export interface IconDefinition {
  paths: string[]
  filled?: boolean
}

const ICONS = {
  search: { paths: [circle(11, 11, 7), 'M 16.2 16.2 L 20.5 20.5'] },
  plus: { paths: ['M 12 5 L 12 19', 'M 5 12 L 19 12'] },
  minus: { paths: ['M 5 12 L 19 12'] },
  check: { paths: ['M 5 12.5 L 9.5 17 L 19 7'] },
  close: { paths: ['M 6 6 L 18 18', 'M 18 6 L 6 18'] },
  chevronDown: { paths: ['M 6 9.5 L 12 15.5 L 18 9.5'] },
  chevronRight: { paths: ['M 9.5 6 L 15.5 12 L 9.5 18'] },
  chevronLeft: { paths: ['M 14.5 6 L 8.5 12 L 14.5 18'] },
  star: {
    paths: [
      'M 12 3.6 L 14.6 9.1 L 20.6 9.9 L 16.2 14.1 L 17.3 20.1 L 12 17.2 L 6.7 20.1 L 7.8 14.1 L 3.4 9.9 L 9.4 9.1 Z',
    ],
  },
  starFilled: {
    paths: [
      'M 12 3.6 L 14.6 9.1 L 20.6 9.9 L 16.2 14.1 L 17.3 20.1 L 12 17.2 L 6.7 20.1 L 7.8 14.1 L 3.4 9.9 L 9.4 9.1 Z',
    ],
    filled: true,
  },
  lock: { paths: [roundedRect(4, 10, 16, 10.5, 2.5), 'M 8 10 V 7.2 a 4 4 0 0 1 8 0 V 10'] },
  unlock: { paths: [roundedRect(4, 10, 16, 10.5, 2.5), 'M 8 10 V 7.2 a 4 4 0 0 1 7.6 -1.7'] },
  shield: { paths: ['M 12 3.2 L 19 6 V 12 c 0 4.4 -2.9 7.6 -7 8.8 -4.1 -1.2 -7 -4.4 -7 -8.8 V 6 Z'] },
  shieldCheck: {
    paths: [
      'M 12 3.2 L 19 6 V 12 c 0 4.4 -2.9 7.6 -7 8.8 -4.1 -1.2 -7 -4.4 -7 -8.8 V 6 Z',
      'M 9 12 l 2.2 2.3 L 15.3 10',
    ],
  },
  key: { paths: [circle(8.5, 15.5, 4), 'M 11.4 12.6 L 20 4', 'M 16.6 7.4 L 18.6 9.4'] },
  note: {
    paths: [
      'M 6.5 3.5 h 8.2 l 4.3 4.3 v 12.7 a 1 1 0 0 1 -1 1 H 6.5 a 1 1 0 0 1 -1 -1 V 4.5 a 1 1 0 0 1 1 -1 z',
      'M 14.5 3.6 V 8 h 4.4',
      'M 9 13 h 6',
      'M 9 16.5 h 4',
    ],
  },
  card: {
    paths: [roundedRect(3, 5.5, 18, 13, 2.5), 'M 3 10 h 18', 'M 7 14.5 h 3.5'],
  },
  copy: {
    paths: [roundedRect(9, 9, 11.5, 11.5, 2.5), 'M 5.5 15 V 6 a 2 2 0 0 1 2 -2 h 9'],
  },
  eye: { paths: ['M 2.5 12 C 4.6 8 8 6 12 6 s 7.4 2 9.5 6 c -2.1 4 -5.5 6 -9.5 6 s -7.4 -2 -9.5 -6 z', circle(12, 12, 2.6)] },
  eyeOff: {
    paths: [
      'M 4 4 L 20 20',
      'M 10.6 6.2 A 9.9 9.9 0 0 1 12 6 c 4 0 7.4 2 9.5 6 a 17 17 0 0 1 -3.3 4.1',
      'M 6.4 8.4 C 4.6 9.4 3.2 10.6 2.5 12 c 2.1 4 5.5 6 9.5 6 a 10 10 0 0 0 3.4 -0.6',
      'M 9.9 9.9 a 3 3 0 0 0 4.2 4.2',
    ],
  },
  refresh: { paths: ['M 20 11.5 A 8 8 0 1 1 17.4 5.9', 'M 20.5 4.5 V 10 h -5.4'] },
  sparkles: {
    paths: [
      'M 11 3.5 L 12.7 8 L 17.2 9.7 L 12.7 11.4 L 11 15.9 L 9.3 11.4 L 4.8 9.7 L 9.3 8 Z',
      'M 17.8 14.2 l 0.8 2 2 0.8 -2 0.8 -0.8 2 -0.8 -2 -2 -0.8 2 -0.8 z',
    ],
  },
  settings: {
    paths: [
      circle(12, 12, 3.1),
      'M 12 3 v 2.2 M 12 18.8 V 21 M 4.9 4.9 l 1.6 1.6 M 17.5 17.5 l 1.6 1.6 M 3 12 h 2.2 M 18.8 12 H 21 M 4.9 19.1 l 1.6 -1.6 M 17.5 6.5 l 1.6 -1.6',
    ],
  },
  trash: {
    paths: [
      'M 4 7 h 16',
      'M 9.5 7 V 5.4 a 1 1 0 0 1 1 -1 h 3 a 1 1 0 0 1 1 1 V 7',
      'M 6.5 7 l 0.9 12.1 a 1 1 0 0 0 1 0.9 h 7.2 a 1 1 0 0 0 1 -0.9 L 17.5 7',
      'M 10.5 11 v 5.5 M 13.5 11 v 5.5',
    ],
  },
  pencil: { paths: ['M 4.5 19.5 h 4 L 19 9 a 2.8 2.8 0 0 0 -4 -4 L 4.5 15.5 z', 'M 13.6 6.4 l 4 4'] },
  clock: { paths: [circle(12, 12, 8.5), 'M 12 7.2 V 12.4 l 3.2 2'] },
  tag: { paths: ['M 3.5 11.2 V 5.5 a 2 2 0 0 1 2 -2 h 5.7 l 9 9 -7.7 7.7 z', circle(7.6, 7.6, 1.3)] },
  globe: { paths: [circle(12, 12, 8.6), 'M 3.4 12 h 17.2', 'M 12 3.4 a 14 14 0 0 1 0 17.2 a 14 14 0 0 1 0 -17.2'] },
  user: { paths: [circle(12, 8, 3.9), 'M 4.4 20.5 c 0 -3.9 3.4 -6 7.6 -6 s 7.6 2.1 7.6 6'] },
  alert: { paths: ['M 12 3.6 L 21 20 H 3 Z', 'M 12 10 v 4', 'M 12 17 h 0.01'] },
  info: { paths: [circle(12, 12, 8.6), 'M 12 11 v 5.5', 'M 12 7.6 h 0.01'] },
  download: { paths: ['M 12 3.5 v 12', 'M 8 11.5 l 4 4 4 -4', 'M 4.5 20 h 15'] },
  upload: { paths: ['M 12 20 V 8', 'M 8 12 l 4 -4 4 4', 'M 4.5 4 h 15'] },
  external: { paths: ['M 13.5 4 H 20 v 6.5', 'M 20 4 L 11.5 12.5', 'M 18 14.5 V 19 a 1 1 0 0 1 -1 1 H 5.5 a 1 1 0 0 1 -1 -1 V 6.5 a 1 1 0 0 1 1 -1 H 10'] },
  sun: {
    paths: [circle(12, 12, 4), 'M 12 2.5 v 2 M 12 19.5 v 2 M 4.2 4.2 l 1.5 1.5 M 18.3 18.3 l 1.5 1.5 M 2.5 12 h 2 M 19.5 12 h 2 M 4.2 19.8 l 1.5 -1.5 M 18.3 5.7 l 1.5 -1.5'],
  },
  moon: { paths: ['M 20.5 15.2 A 8.6 8.6 0 0 1 8.8 3.5 a 8.8 8.8 0 1 0 11.7 11.7 z'] },
  monitor: { paths: [roundedRect(3.5, 5, 17, 11.5, 2), 'M 2.5 19.5 h 19'] },
  cornerArrow: { paths: ['M 9 6 l -4 6 4 6', 'M 5 12 h 9 a 5 5 0 0 1 5 5 v 0'] },
  dice: {
    paths: [
      roundedRect(3.5, 3.5, 17, 17, 4),
      'M 8.5 8.6 h 0.01',
      'M 15.5 8.6 h 0.01',
      'M 12 12 h 0.01',
      'M 8.5 15.4 h 0.01',
      'M 15.5 15.4 h 0.01',
    ],
  },
  history: { paths: [circle(12, 12, 8.6), 'M 12 7.5 V 12 l 3 1.8', 'M 3.6 8.5 h 3.2'] },
  reveal: { paths: [circle(12, 12, 8.6), 'M 12 8 v 8', 'M 8 12 h 8'] },
  laptop: { paths: [roundedRect(4.5, 5, 15, 10.5, 1.8), 'M 2.5 18.8 h 19'] },
  filter: { paths: ['M 4 6 h 16', 'M 7 12 h 10', 'M 10 18 h 4'] },
} as const satisfies Record<string, IconDefinition>

export type IconName = keyof typeof ICONS

export function iconPaths(name: IconName): IconDefinition {
  return ICONS[name]
}
