#!/usr/bin/env perl
use strict;
use warnings;
use Getopt::Long qw/:config posix_default no_ignore_case bundling permute/;

my %opt = (
    func   => '',
    str    => '',
    unique => 0,
);
GetOptions(
    'func=s'    => \$opt{func},
    'str=s'     => \$opt{str},
    'u|unique!' => \$opt{unique},
) or usage();

my $file = shift or usage();

my $syms = `readelf -sW $file`;
my $offset = `strings -tx $file`;
my (%func, %str);

offset('__libc_start_main');
offset('system');
offset_str('/bin/sh');

offset($_) for split /,/, $opt{func};
offset_str($_) for split /,/, $opt{str};

print "offset = {\n";
if (!$opt{unique}) {
    %func = invert_hash(invert_hash(%func));
    %str = invert_hash(invert_hash(%str));
}
for my $k (sort { $func{$a} cmp $func{$b} or $a <=> $b } keys %func) {
    printf "    '%s': 0x%x,\n", $func{$k}, $k;
}
for my $k (sort { $str{$a} cmp $str{$b} or $a <=> $b } keys %str) {
    printf "    '%s': 0x%x, # str\n", $str{$k}, $k;
}
print "}\n";

sub invert_hash {
    my (%a) = @_;
    my %ret;
    for my $k (keys %a) {
        $ret{$a{$k}} = $k;
    }
    return %ret;
}

sub offset {
    my ($func) = @_;
    while ($syms =~ /\d+:\s+([0-9a-f]+).+FUNC.+\s+$func@@/g) {
        $func{hex $1} = $func;
    }
}

sub offset_str {
    my ($str) = @_;
    while ($offset =~ /\s+([0-9a-f]+)\s+$str/g) {
        $str{hex $1} = $str;
    }
}

sub usage {
    die "Usage: $0 FILE\n";
}
