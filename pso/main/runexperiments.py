#!/usr/bin/python

from __future__ import print_function, division

import argparse
import datetime
import hashlib
import os
import os.path
import subprocess
import sys
import time

num_samples = 20

def gen_experiments():
  experiments={
    "fit": ["parabola:30:0.25",
            "rastrigin:30:0.25",
            "rosenbrock:30:0.25",
            "ackley:30:0.25",
           ],
    "mdecay": 1.0,
    ("mtype", "ttype", "m0", "m1", "sc", "cc", "sclb", "cclb", "bcog"): [
      ("linear", "none", "0.8", "0.5", "2.05", "2.05", "0.0", "0.0", None),
      ("linear", "rtrunc", "0.8", "0.5", "2.05", "2.05", "0.0", "0.0", None),

      ("linear", "none", "0", "0", "2.05", "2.05", "-1", "-1", None),
      ("linear", "none", "0", "0", "2.05", "2.05", "-1", "-1", True),

      ("constant", "none", "0.72984", "0.72984", "1.5", "1.5", "0.0", "0.0", None), # equivalent to standard constriction
      ("constant", "rtrunc", "0.72984", "0.72984", "1.5", "1.5", "0.0", "0.0", None), # equivalent to standard constriction
      ("constant", "none", "0.0", "0.0", "1.5", "1.5", "0.0", "0.0", None),
      ("constant", "none", "0.0", "0.0", "2.05", "2.05", "0.0", "0.0", None),
      ("constant", "none", "0.0", "0.0", "1.5", "1.5", "-0.1", "-0.1", None),
      ("constant", "none", "0.0", "0.0", "2.05", "2.05", "-0.1", "-0.1", None),
      ("constant", "none", "0.0", "0.0", "1.5", "1.5", "-0.15", "-0.15", None),
      ("constant", "none", "0.0", "0.0", "2.05", "2.05", "-0.15", "-0.15", None),
    ],
    "rmul": ["0.0",
             "0.1"],
    "cdecay": ["1.0",
               "0.99"],
    "n": ["300000",
          "500000"],
    "outputfreq": "1000",
    "rdecay": "0.9",
    "topo": "ring:20",
  }

  items = experiments.items()
  combos = gen_combinations(len(v) for _, v in items)

  # Clean up the values to make everything consistently structured.
  for i, (k, values) in enumerate(items):
    if isinstance(k, (str, unicode)):
      k = [k]
      if not isinstance(values, (list, tuple)):
        values = [values]
    if len(k) == 1:
      values = [x if isinstance(x, (list, tuple)) else [x] for x in values]
    items[i] = (k, values)

  def iterflags(combo):
    for flags, values in ((f, v[i]) for (f, v), i in zip(items, combo)):
      if len(flags) != len(values):
        raise ValueError("Invalid experiment setting: %r:%r" % flags, values)
      for flag, setting in zip(flags, values):
        if setting is None: # fall back to default value, skip flag in this case.
          continue
        if isinstance(setting, bool):
          if setting:
            yield "-%s" % flag
          else:
            yield "-no%s" % flag
        else:
          yield "-%s=%s" % (flag, setting)

  for indices in combos:
    yield sorted(iterflags(indices))


def gen_combinations(counts):
  """Generate all combinations of item indices, assuming counts describes list lengths.

  Args:
    counts: an iterable over item counts.

  Yields: lists of indices, all permutations.
  """
  counts = tuple(counts)
  for c in counts:
    if c <= 0:
      raise ValueError("Counts contains non-positive value: " + str(counts))

  cur = [0] * len(counts)
  while True:
    yield tuple(cur)
    for i in xrange(len(counts)-1, -1, -1):
      cur[i] += 1
      if cur[i] < counts[i]:
        break
      cur[i] = 0
    else:
      break


SKIP="skip"
RERUN="rerun"
RUN="run"

def runstate(outname):
  """Check the state of the currently desired output file. Returns

  Args:
    outname: the name of the output file.

  Returns:
    one of RUN, SKIP, or RERUN, depending on what needs to happen next.
  """
  DONE_STR = "# DONE"
  if os.path.exists(outname):
    with open(outname) as f:
      try:
        f.seek(-len(DONE_STR)-2, 2) # Add byte for the bounding newlines
      except IOError:
        print("Error in seeking for file {}".format(outname))
        sys.exit(1)
      if "\n" + DONE_STR not in f.read():
        return RERUN
      else:
        return SKIP
  return RUN


def run(progname, flags, outname):
  with open(outname, "w") as f:
    print("# {flags}".format(flags=" ".join(flags)), file=f)
    # "print" buffers its output, so we have to flush this before calling the
    # subprocess and expecting it to be in the right order.
    f.flush()
    returncode = subprocess.call([progname] + list(flags), stdout=f)
    if returncode == 0:
      print("# DONE", file=f)
      f.flush()
      return True
    return False


def main():
  parser = argparse.ArgumentParser(description="Run experiments.")
  parser.add_argument('-n', '--dry', action="store_true", help="Show changes without running.")
  parser.add_argument('-r', '--runner', type=str, help="Location of the binary to run.")
  args = parser.parse_args()

  to_run = []
  total = 0
  for s in xrange(num_samples):
    for flags in gen_experiments():
      flagstr = " ".join(flags)
      m = hashlib.md5()
      m.update(flagstr)
      dig = m.hexdigest()
      out = "out/exp-{digest}-{sample:02d}".format(digest=dig, sample=s)

      total += 1

      state = runstate(out)
      if state == SKIP:
        print("S {out}".format(out=out))
        continue
      elif state == RERUN:
        print("M {out}".format(out=out))
      elif state == RUN:
        print("A {out}".format(out=out))

      to_run.append((state, flags, out))

  print("--------------------------------------------------")
  print("RUNNING {:d} of {:d} experiments".format(len(to_run), total))
  print("--------------------------------------------------")

  if args.dry:
    print("Dry run: not running experiments.")
    return

  avg_duration = None

  progname = os.path.expanduser(args.runner or "./main")

  for i, (state, flags, out) in enumerate(to_run):
    print("RUNNING {iter:d}/{total:d} (state={state}) {out}: {prog} {flags}".format(state=state, flags=" ".join(flags), out=out, prog=progname, iter=i+1, total=len(to_run)))
    start_time = time.time()
    if not run(progname, flags, out):
      print("Failed to run {} {}".format(progname, flags))
      sys.exit(1)
    duration = time.time() - start_time
    if avg_duration is None:
      avg_duration = duration * 1.2
    else:
      avg_duration += 0.1 * (duration - avg_duration)
    estimated_remaining = (len(to_run)-i-1) * avg_duration
    print("  ** Completed in {:.1f} seconds. Estimated time remaining: {:s}".format(
      duration, str(datetime.timedelta(seconds=estimated_remaining)).split('.')[0]))


#------------------------------------------------------------------------------
if __name__ == '__main__':
  main()
