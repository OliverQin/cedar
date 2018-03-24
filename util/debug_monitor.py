#!/usr/bin/env python3
# -*- coding:UTF-8

import os
from IPython import embed

def load_debug_file(filename):
    hash_map = {}
    id_map = {}

    with open(filename) as f:
        for line in f:
            line = line.strip()

            if ' [Debug] ' not in line or line.count('[') != 2:
                continue
            
            timestamp, others = line.split(' [Debug] ')
            timestamp = float(timestamp)
            tp = tuple(others.split())

            hash_val, idx = None, None
            ntp = (timestamp, )
            for i in tp:
                if i.startswith('[') and i.endswith(']'):
                    tag = i[1:-1]
                    ntp = ntp + (tag, )
                elif i.startswith('sh'):
                    hash_val = i
                    ntp = ntp + (hash_val, )
                else:
                    idx = int(i)
                    ntp = ntp + (idx, )
            
            assert (hash_val is not None or idx is not None)

            if hash_val and idx is None:
                if hash_val not in hash_map:
                    hash_map[hash_val] = []
                ret = hash_map.get(hash_val)
            elif hash_val is None and idx:
                if idx not in id_map:
                    id_map[idx] = []
                ret = id_map.get(idx)
            else:
                if hash_val not in hash_map:
                    hash_map[hash_val] = []
                if idx not in id_map:
                    id_map[idx] = hash_map[hash_val]

                a = hash_map.get(hash_val)
                b = id_map.get(idx)
                if a is not b:
                    a = a + b
                    id_map[idx] = a
                    hash_map[hash_val] = a
                    id_map[idx].sort()

                ret = a
            
            ret.append(ntp)

    return hash_map, id_map


if __name__ == '__main__':
    hmap, idmap = load_debug_file(os.path.expanduser("~/go/src/github.com/OliverQin/cedar/bin/test_comb/d.log"))
    embed()

            

                