"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from "react";

import { cn } from "@/lib/utils";

type ShuffleProps = {
  text: string;
  className?: string;
  style?: CSSProperties;
  duration?: number;
  stagger?: number;
  shuffleTimes?: number;
  scrambleCharset?: string;
  colorFrom?: string;
  colorTo?: string;
  triggerOnHover?: boolean;
  respectReducedMotion?: boolean;
};

const DEFAULT_CHARSET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
const SHUFFLE_EASING = "cubic-bezier(0.16, 1, 0.3, 1)";

function seededChar(charset: string, charCode: number, charIndex: number, roll: number, playKey: number) {
  const seed = charCode * 31 + charIndex * 97 + roll * 53 + playKey * 131;
  return charset[Math.abs(seed) % charset.length] || "";
}

function displayChar(char: string) {
  return char === " " ? "\u00A0" : char;
}

export function Shuffle({
  text,
  className,
  style,
  duration = 0.62,
  stagger = 0.035,
  shuffleTimes = 3,
  scrambleCharset = DEFAULT_CHARSET,
  colorFrom,
  colorTo,
  triggerOnHover = true,
  respectReducedMotion = true,
}: ShuffleProps) {
  const [playKey, setPlayKey] = useState(0);
  const [reducedMotion, setReducedMotion] = useState(false);
  const [charWidths, setCharWidths] = useState<number[]>([]);
  const innerRefs = useRef<(HTMLSpanElement | null)[]>([]);
  const finalCharRefs = useRef<(HTMLSpanElement | null)[]>([]);
  const playingRef = useRef(false);
  const characters = useMemo(() => Array.from(text), [text]);

  const sequences = useMemo(() => {
    const rolls = Math.max(1, Math.floor(shuffleTimes));
    return characters.map((char, charIndex) => {
      if (char === " ") return [char];
      const code = char.codePointAt(0) ?? charIndex;
      const shuffled = Array.from({ length: rolls }, (_, roll) =>
        seededChar(scrambleCharset, code, charIndex, roll, playKey),
      );
      return [...shuffled, char];
    });
  }, [characters, playKey, scrambleCharset, shuffleTimes]);

  const measureWidths = useCallback(() => {
    const nextWidths = finalCharRefs.current.map((node) => {
      if (!node) return 0;
      return node.getBoundingClientRect().width;
    });
    setCharWidths((prev) => {
      if (
        prev.length === nextWidths.length &&
        prev.every((width, index) => Math.abs(width - nextWidths[index]) < 0.5)
      ) {
        return prev;
      }
      return nextWidths;
    });
  }, []);

  useLayoutEffect(() => {
    measureWidths();
  }, [measureWidths, sequences]);

  useEffect(() => {
    if (!("fonts" in document)) return undefined;
    let cancelled = false;
    document.fonts.ready.then(() => {
      if (!cancelled) measureWidths();
    });
    return () => {
      cancelled = true;
    };
  }, [measureWidths]);

  useEffect(() => {
    const onResize = () => measureWidths();
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [measureWidths]);

  useEffect(() => {
    if (!respectReducedMotion) return undefined;
    const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
    const updateMotionPreference = () => setReducedMotion(motionQuery.matches);

    updateMotionPreference();
    motionQuery.addEventListener("change", updateMotionPreference);
    return () => motionQuery.removeEventListener("change", updateMotionPreference);
  }, [respectReducedMotion]);

  useEffect(() => {
    const animations: Animation[] = [];
    playingRef.current = true;

    innerRefs.current.forEach((inner, index) => {
      if (!inner) return;
      const steps = Math.max(0, (sequences[index]?.length ?? 1) - 1);
      const finalTransform = `translate3d(0, -${steps}em, 0)`;

      if (reducedMotion || steps === 0) {
        inner.style.transform = finalTransform;
        inner.style.willChange = "auto";
        return;
      }

      inner.style.transform = "translate3d(0, 0, 0)";
      inner.style.willChange = "transform";
      const animation = inner.animate(
        [
          { transform: "translate3d(0, 0, 0)" },
          { transform: finalTransform },
        ],
        {
          duration: duration * 1000,
          delay: index * stagger * 1000,
          easing: SHUFFLE_EASING,
          fill: "forwards",
        },
      );
      animations.push(animation);
    });

    if (animations.length === 0) {
      playingRef.current = false;
      return undefined;
    }

    Promise.allSettled(animations.map((animation) => animation.finished)).then(() => {
      playingRef.current = false;
      animations.forEach((animation) => {
        const effect = animation.effect as KeyframeEffect | null;
        const target = effect?.target as HTMLElement | null;
        if (target) {
          const index = innerRefs.current.indexOf(target as HTMLSpanElement);
          const steps = Math.max(0, (sequences[index]?.length ?? 1) - 1);
          target.style.transform = `translate3d(0, -${steps}em, 0)`;
          target.style.willChange = "auto";
        }
        animation.cancel();
      });
    });

    return () => {
      animations.forEach((animation) => animation.cancel());
      playingRef.current = false;
    };
  }, [duration, reducedMotion, sequences, stagger]);

  const replay = () => {
    if (!triggerOnHover || playingRef.current || reducedMotion) return;
    setPlayKey((key) => key + 1);
  };

  return (
    <span
      className={cn("inline-flex whitespace-nowrap leading-none", className)}
      style={{ color: colorTo, ...style }}
      onMouseEnter={replay}
    >
      {sequences.map((sequence, charIndex) => {
        const finalIndex = sequence.length - 1;
        return (
          <span
            key={`${characters[charIndex]}-${charIndex}-${playKey}`}
            className="inline-block h-[1em] overflow-hidden align-baseline leading-none"
            style={{
              width: charWidths[charIndex] ? `${charWidths[charIndex]}px` : undefined,
            }}
          >
            <span
              ref={(node) => {
                innerRefs.current[charIndex] = node;
              }}
              className="block leading-none"
            >
              {sequence.map((char, sequenceIndex) => (
                <span
                  key={`${char}-${sequenceIndex}`}
                  ref={(node) => {
                    if (sequenceIndex === finalIndex) finalCharRefs.current[charIndex] = node;
                  }}
                  className="block h-[1em] text-center leading-none"
                  style={{
                    color: sequenceIndex === finalIndex ? colorTo : colorFrom,
                  }}
                >
                  {displayChar(char)}
                </span>
              ))}
            </span>
          </span>
        );
      })}
    </span>
  );
}
