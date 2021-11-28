#!/usr/bin/env python3

import argparse
import binascii
import json
import struct
import subprocess
import sys


class FatalError(Exception):
    pass


class ImageSegment(object):
    """ Wrapper class for a segment in an ESP image
    (very similar to a section in an ELFImage also) """
    def __init__(self, addr, data, file_offs=None):
        self.addr = addr
        # pad all ImageSegments to at least 4 bytes length
        pad_mod = len(data) % 4
        if pad_mod != 0:
            data += b"\x00" * (4 - pad_mod)
        self.data = data
        self.file_offs = file_offs
        self.include_in_checksum = True

    def copy_with_new_addr(self, new_addr):
        """ Return a new ImageSegment with same data, but mapped at
        a new address. """
        return ImageSegment(new_addr, self.data, 0)

    def __repr__(self):
        r = "len 0x%05x load 0x%08x" % (len(self.data), self.addr)
        if self.file_offs is not None:
            r += " file_offs 0x%08x" % (self.file_offs)
        return r


class ELFSection(ImageSegment):
    """ Wrapper class for a section in an ELF image, has a section
    name as well as the common properties of an ImageSegment. """
    def __init__(self, name, addr, data):
        super(ELFSection, self).__init__(addr, data)
        self.name = name

    def __repr__(self):
        return "%s %s" % (self.name, super(ELFSection, self).__repr__())


class ELFFile(object):
    SEC_TYPE_PROGBITS = 0x01
    SEC_TYPE_STRTAB = 0x03

    def __init__(self, name):
        # Load sections from the ELF file
        self.name = name
        self.symbols = None
        with open(self.name, 'rb') as f:
            self._read_elf_file(f)

    def get_section(self, section_name):
        for s in self.sections:
            if s.name == section_name:
                return s
        raise ValueError("No section %s in ELF file" % section_name)

    def _read_elf_file(self, f):
        # read the ELF file header
        LEN_FILE_HEADER = 0x34
        try:
            (ident,_type,machine,_version,
             self.entrypoint,_phoff,shoff,_flags,
             _ehsize, _phentsize,_phnum,_shentsize,
             _shnum,shstrndx) = struct.unpack("<16sHHLLLLLHHHHHH", f.read(LEN_FILE_HEADER))
        except struct.error as e:
            raise FatalError("Failed to read a valid ELF header from %s: %s" % (self.name, e))

        if ident[0] != 0x7f or ident[1:4] != b'ELF':
            raise FatalError("%s has invalid ELF magic header %s" % (self.name, ident))
        if machine != 0x5e and machine != 0xf3:
            raise FatalError("%s does not appear to be an Xtensa ELF file. e_machine=%04x" % (self.name, machine))
        self._read_sections(f, shoff, shstrndx)

    def _read_sections(self, f, section_header_offs, shstrndx):
        f.seek(section_header_offs)
        section_header = f.read()
        LEN_SEC_HEADER = 0x28
        if len(section_header) == 0:
            raise FatalError("No section header found at offset %04x in ELF file." % section_header_offs)
        if len(section_header) % LEN_SEC_HEADER != 0:
            print('WARNING: Unexpected ELF section header length %04x is not mod-%02x' % (len(section_header),LEN_SEC_HEADER))

        # walk through the section header and extract all sections
        section_header_offsets = range(0, len(section_header), LEN_SEC_HEADER)

        def read_section_header(offs):
            name_offs,sec_type,_flags,lma,sec_offs,size = struct.unpack_from("<LLLLLL", section_header[offs:])
            return (name_offs, sec_type, lma, size, sec_offs)
        all_sections = [read_section_header(offs) for offs in section_header_offsets]
        prog_sections = [s for s in all_sections if s[1] == ELFFile.SEC_TYPE_PROGBITS]

        # search for the string table section
        if not shstrndx * LEN_SEC_HEADER in section_header_offsets:
            raise FatalError("ELF file has no STRTAB section at shstrndx %d" % shstrndx)
        _,sec_type,_,sec_size,sec_offs = read_section_header(shstrndx * LEN_SEC_HEADER)
        if sec_type != ELFFile.SEC_TYPE_STRTAB:
            print('WARNING: ELF file has incorrect STRTAB section type 0x%02x' % sec_type)
        f.seek(sec_offs)
        string_table = f.read(sec_size)

        # build the real list of ELFSections by reading the actual section names from the
        # string table section, and actual data for each section from the ELF file itself
        def lookup_string(offs):
            raw = string_table[offs:]
            return raw[:raw.index(b'\x00')].decode("ascii")

        def read_data(offs,size):
            f.seek(offs)
            return f.read(size)

        prog_sections = [ELFSection(lookup_string(n_offs), lma, read_data(offs, size)) for (n_offs, _type, lma, size, offs) in prog_sections
                         if lma != 0]
        self.sections = prog_sections

    def _fetch_symbols(self):
        if self.symbols is not None:
            return
        self.symbols = {}
        try:
            tool_nm = "nm"
            proc = subprocess.Popen([tool_nm, self.name], stdout=subprocess.PIPE)
        except OSError:
            print("Error calling %s, do you have Xtensa toolchain in PATH?" % tool_nm)
            sys.exit(1)
        for l in proc.stdout:
            fields = l.strip().split()
            try:
                if fields[0] == "U":
                    print("Warning: ELF binary has undefined symbol %s" % fields[1])
                    continue
                if fields[0] == "w":
                    continue  # can skip weak symbols
                self.symbols[fields[2].decode("ascii")] = int(fields[0], 16)
            except ValueError:
                raise FatalError("Failed to strip symbol output from nm: %s" % fields)

    def get_symbol_addr(self, sym):
        self._fetch_symbols()
        return self.symbols[sym]


def wrap_stub(args):
    e = ELFFile(args.input)

    stub = {
        'params_start': e.get_symbol_addr('_params_start'),
        'code': e.get_section('.text').data,
        'code_start': e.get_symbol_addr('_code_start'),
        'entry': e.get_symbol_addr(args.entry),
    }
    try:
        stub['data'] = e.get_section('.data').data
        stub['data_start'] = e.get_symbol_addr('_data_start')
    except ValueError:
        pass
        # No data section, that's fine
    bss_size, bss_start = 0, 0
    try:
        bss_start = e.get_symbol_addr('_bss_start')
        bss_size = e.get_symbol_addr('_bss_end') - bss_start
    except ValueError:
        pass
    params_len = e.get_symbol_addr('_params_end') - stub['params_start']
    if params_len % 4 != 0:
        raise FatalError('Params must be dwords')
    stub['num_params'] = int(params_len / 4)

    # Pad code with NOPs to mod 4.
    if len(stub['code']) % 4 != 0:
        stub['code'] += (4 - (len(stub['code']) % 4)) * '\0'

    print(
            'Stub params: %d @ 0x%08x, code: %d @ 0x%08x, bss: %d @ 0x%08x, data: %d @ 0x%08x, entry: %s @ 0x%x' % (
            params_len, stub['params_start'],
            len(stub['code']), stub['code_start'],
            bss_size, bss_start,
            len(stub.get('data', '')), stub.get('data_start', 0),
            args.entry, stub['entry']),
            file=sys.stderr)

    jstub = dict(stub)
    jstub['code_size'] = len(stub['code'])
    jstub['code'] = binascii.hexlify(stub['code']).decode("ascii")
    if 'data' in stub:
        jstub['data_size'] = len(stub['data'])
        jstub['data'] = binascii.hexlify(stub['data']).decode("ascii")
    if args.output:
        with open(args.output, "w") as f:
            json.dump(jstub, f)
    else:
        print(json.dumps(jstub))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Wrap stub and output a JSON object", prog="wrap_stub")
    parser.add_argument('--entry', default='stub_main')
    parser.add_argument('--output', default=None)
    parser.add_argument('input')
    args = parser.parse_args()
    wrap_stub(args)
