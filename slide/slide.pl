use strict;
use warnings;
use Text::Xslate;
use Text::MultiMarkdown qw/markdown/;
use Data::Section::Simple qw/get_data_section/;
use Pod::Usage;

my $file = shift or pod2usage(1);

my $content = do {
    open my $fh, '<', $file or die $!;
    local $/; <$fh>;
};

my @slides = map { markdown($_) } split /\n\n\n/, $content;
my ($title) = $slides[0] =~ m!<h1>(.*?)</h1>!;

my $tx = Text::Xslate->new();
print $tx->render_string(get_data_section('slide.tx'), {
    title => $title,
    slides => \@slides,
});

__END__

=head1 SYNOPSIS

    % slide.pl file

=cut

__DATA__

@@ slide.tx
<!doctype html>
<html>
<head>
    <meta charset="utf-8" />
    <title><: $title :></title>
    <link rel="stylesheet" href="static/css/screen.css" type="text/css" media="screen, projection" />
    <link rel="stylesheet" href="static/css/prettify.css" type="text/css" />
    <link rel="stylesheet" href="static/css/slide.css" type="text/css" />
    <script type="text/javascript" src="static/js/jquery-1.6.2.min.js"></script>
    <script type="text/javascript" src="static/js/jquery.presentation.js"></script>
    <script type="text/javascript" src="static/js/prettify/prettify.js"></script>
</head>
<body>
    <div id="slides">
    : for $slides -> $slide {
        <div class="slide">
<: $slide | mark_raw :>        </div>
    : }
    </div>
    <script>
$(function () {
    $(document).ready(function () {
        $('pre').addClass('prettyprint');
        prettyPrint();
    });

    $('#slides').presentation();
});
    </script>
</body>
</html>
