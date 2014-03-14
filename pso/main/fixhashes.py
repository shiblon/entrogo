#!/usr/bin/python

from __future__ import division, print_function

import hashlib
import os
import os.path

def main():
  dirname = "out"
  for name in os.listdir(dirname):
    prefix, name_dig, iteration = name.split('-')
    fname = os.path.join(dirname, name)
    line = open(fname).readline()
    if not line.startswith('#'):
      print("E {}: incorrect first line {!r}".format(fname, line))
      continue
    line = line.lstrip('# ')
    canonical_flags = " ".join(sorted(line.split()))
    m = hashlib.md5()
    m.update(canonical_flags)
    calc_dig = m.hexdigest()
    if calc_dig == name_dig:
      print("S {}".format(fname))
    else:
      new_fname = os.path.join(dirname, "-".join((prefix, calc_dig, iteration)))
      print("R {}: => {}".format(fname, new_fname))
      os.rename(fname, new_fname)

if __name__ == '__main__':
  main()
