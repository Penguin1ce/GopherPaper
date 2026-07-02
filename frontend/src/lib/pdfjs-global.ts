import * as pdfjsLib from "pdfjs-dist";

const pdfjsGlobal = globalThis as typeof globalThis & {
  pdfjsLib?: typeof pdfjsLib;
};

pdfjsGlobal.pdfjsLib ??= pdfjsLib;

