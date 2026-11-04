// The audio fakes every screen test plays through (setup.ts points the shared player at them).
import { fakeEnv } from "../audio/fakes";

export const audio = fakeEnv();
